package core

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-zookeeper/zk"
)

const flag int32 = 0

var acl = zk.WorldACL(zk.PermAll)
var ErrUnknownCmd = errors.New("unknown command")

type Cmd struct {
	Name        string
	Options     []string
	ExitWhenErr bool
	Conn        *zk.Conn
	Config      *Config
}

func NewCmd(name string, options []string, conn *zk.Conn, config *Config) *Cmd {
	return &Cmd{
		Name:    name,
		Options: options,
		Conn:    conn,
		Config:  config,
	}
}

func ParseCmd(cmd string) (name string, options []string) {
	args := make([]string, 0)
	for _, cmd := range strings.Split(cmd, " ") {
		if cmd != "" {
			args = append(args, cmd)
		}
	}
	if len(args) == 0 {
		return
	}

	return args[0], args[1:]
}

func (c *Cmd) ls() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	p := "/"
	options := c.Options
	if len(options) > 0 {
		p = options[0]
	}
	p = cleanPath(p)
	children, _, err := c.Conn.Children(p)
	if err != nil {
		return
	}
	fmt.Printf("[%s]\n", strings.Join(children, ", "))
	return
}

func (c *Cmd) get() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	p := "/"
	options := c.Options
	if len(options) > 0 {
		p = options[0]
	}
	p = cleanPath(p)
	value, stat, err := c.Conn.Get(p)
	if err != nil {
		return
	}
	fmt.Printf("%+v\n%s\n", string(value), fmtStat(stat))
	return
}

func (c *Cmd) create() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	p := "/"
	data := ""
	options := c.Options
	if len(options) > 0 {
		p = options[0]
		if len(options) > 1 {
			data = options[1]
		}
	}
	p = cleanPath(p)
	_, err = c.Conn.Create(p, []byte(data), flag, acl)
	if err != nil {
		return
	}
	fmt.Printf("Created %s\n", p)
	root, _ := splitPath(p)
	suggestCache.del(root)
	return
}

func (c *Cmd) set() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	p := "/"
	data := ""
	options := c.Options
	if len(options) > 0 {
		p = options[0]
		if len(options) > 1 {
			data = options[1]
		}
	}
	p = cleanPath(p)
	stat, err := c.Conn.Set(p, []byte(data), -1)
	if err != nil {
		return
	}
	fmt.Printf("%s\n", fmtStat(stat))
	return
}

func (c *Cmd) delete() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	p := "/"
	options := c.Options
	if len(options) > 0 {
		p = options[0]
	}
	p = cleanPath(p)
	err = c.Conn.Delete(p, -1)
	if err != nil {
		return
	}
	fmt.Printf("Deleted %s\n", p)
	root, _ := splitPath(p)
	suggestCache.del(root)
	return
}

func deleteChildren(c *Cmd, path string) (err error) {
	children, _, err := c.Conn.Children(path)
	if err != nil {
		return
	}

	ops := []interface{}{}

	for _, child := range children {
		path := fmt.Sprintf("%s/%s", path, child)

		err = deleteChildren(c, path)

		ops = append(ops, &zk.DeleteRequest{Path: path})

		if err != nil {
			return
		}
		fmt.Printf("Deleting %s\n", path)

		if len(ops) == 2000 {
			_, err = c.Conn.Multi(ops...)
			if err != nil {
				return
			}

			ops = []interface{}{}
		}
	}

	if len(ops) > 0 {
		_, err = c.Conn.Multi(ops...)
		if err != nil {
			return
		}
	}

	root, _ := splitPath(path)
	suggestCache.del(root)
	return
}

func (c *Cmd) deleteall() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	p := "/"

	options := c.Options
	if len(options) > 0 {
		p = options[0]
	}
	p = cleanPath(p)

	if p == "/" {
		return errors.New("cannot use root")
	}

	if p == "/clickhouse" {
		return errors.New("cannot use /clickhouse")
	}

	if p == "/clickhouse/tables" {
		return errors.New("cannot use /clickhouse/tables")
	}

	if !strings.HasPrefix(p, "/clickhouse/backups") || p == "/clickhouse/backups" {
		return errors.New("only paths starting with /clickhouse/backups allowed")
	}

	err = deleteChildren(c, p)
	if err != nil {
		return
	}

	err = c.Conn.Delete(p, -1)
	if err != nil {
		return
	}

	root, _ := splitPath(p)
	suggestCache.del(root)

	return
}

func (c *Cmd) deletestalebackups() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	backup_root := "/clickhouse/backups"

	backups, _, err := c.Conn.Children(backup_root)
	if err != nil {
		return
	}

	for _, child := range backups {
		backup_path := fmt.Sprintf("%s/%s", backup_root, child)

		fmt.Printf("Found backup %s, checking if it's active\n", backup_path)

		stage_path := fmt.Sprintf("%s/stage", backup_path)

		var stages []string
		stages, _, err = c.Conn.Children(stage_path)
		if err != nil && err != zk.ErrNoNode {
			return
		}

		var is_active = false
		for _, stage := range stages {
			if strings.HasPrefix(stage, "alive") {
				is_active = true
				break
			}
		}

		if is_active {
			fmt.Printf("Backup %s is active, not going to delete\n", backup_path)
			continue
		}

		fmt.Printf("Backup %s is not active, deleting it\n", backup_path)
		err = deleteChildren(c, backup_path)
		if err != nil {
			return
		}

		err = c.Conn.Delete(backup_path, -1)
		if err != nil {
			return
		}

	}

	return
}

func (c *Cmd) close() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	c.Conn.Close()
	if !c.connected() {
		fmt.Println("Closed")
	}
	return
}

func (c *Cmd) connect() (err error) {
	options := c.Options
	var conn *zk.Conn
	if len(options) > 0 {
		cf := NewConfig(strings.Split(options[0], ","), false)
		conn, err = cf.Connect()
		if err != nil {
			return err
		}
	} else {
		conn, err = c.Config.Connect()
		if err != nil {
			return err
		}
	}
	if c.connected() {
		c.Conn.Close()
	}
	c.Conn = conn
	fmt.Println("Connected")
	return err
}

func (c *Cmd) addAuth() (err error) {
	err = c.checkConn()
	if err != nil {
		return
	}

	options := c.Options
	if len(options) < 2 {
		return errors.New("addauth <scheme> <auth>")
	}
	scheme := options[0]
	auth := options[1]
	err = c.Conn.AddAuth(scheme, []byte(auth))
	if err != nil {
		return
	}
	fmt.Println("Added")
	return err
}

func (c *Cmd) connected() bool {
	state := c.Conn.State()
	return state == zk.StateConnected || state == zk.StateHasSession
}

func (c *Cmd) checkConn() (err error) {
	if !c.connected() {
		err = errors.New("connection is disconnected")
	}
	return
}

func (c *Cmd) run() (err error) {
	switch c.Name {
	case "ls":
		return c.ls()
	case "get":
		return c.get()
	case "create":
		return c.create()
	case "set":
		return c.set()
	case "delete":
		return c.delete()
	case "deleteall":
		return c.deleteall()
	case "deletestalebackups":
		return c.deletestalebackups()
	case "close":
		return c.close()
	case "connect":
		return c.connect()
	case "addauth":
		return c.addAuth()
	default:
		return ErrUnknownCmd
	}
}

func (c *Cmd) Run() {
	err := c.run()
	if err != nil {
		if err == ErrUnknownCmd {
			printHelp()
			if c.ExitWhenErr {
				os.Exit(2)
			}
		} else {
			printRunError(err)
			if c.ExitWhenErr {
				os.Exit(3)
			}
		}
	}
}

func printHelp() {
	fmt.Println(`get <path>
ls <path>
create <path> [<data>]
set <path> [<data>]
delete <path>
deleteall <path>
deletestalebackups
connect <host:port>
addauth <scheme> <auth>
close
exit`)
}

func printRunError(err error) {
	fmt.Println(err)
}

func cleanPath(p string) string {
	if p == "/" {
		return p
	}
	return strings.TrimRight(p, "/")
}

func GetExecutor(cmd *Cmd) func(s string) {
	return func(s string) {
		name, options := ParseCmd(s)
		cmd.Name = name
		cmd.Options = options
		if name == "exit" {
			os.Exit(0)
		}
		cmd.Run()
	}
}
