package zkcli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/samuel/go-zookeeper/zk"
)

const flag int32 = 0

var acl = zk.WorldACL(zk.PermAll)
var ErrUnknownCmd = errors.New("unknown command")

type Cmd struct {
	Name        string
	Options     []string
	ExitWhenErr bool
	Conn        *zk.Conn
}

func ParseCmd(cmd string) *Cmd {
	arr := strings.Split(cmd, " ")
	cmds := []string{}
	for _, cmd := range arr {
		if cmd != "" {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}

	return &Cmd{
		Name:    cmds[0],
		Options: cmds[1:],
	}
}

func (c *Cmd) ls(conn *zk.Conn) (err error) {
	path := "/"
	options := c.Options
	if len(options) > 0 {
		path = options[0]
	}
	children, _, err := conn.Children(path)
	if err != nil {
		return
	}
	fmt.Printf("[%s]\n", strings.Join(children, ", "))
	return
}

func (c *Cmd) get(conn *zk.Conn) (err error) {
	path := "/"
	options := c.Options
	if len(options) > 0 {
		path = options[0]
	}
	value, stat, err := conn.Get(path)
	if err != nil {
		return
	}
	fmt.Printf("%+v\n%s\n", string(value), fmtStat(stat))
	return
}

func (c *Cmd) create(conn *zk.Conn) (err error) {
	path := "/"
	data := ""
	options := c.Options
	if len(options) > 0 {
		path = options[0]
		if len(options) > 1 {
			data = options[1]
		}
	}
	_, err = conn.Create(path, []byte(data), flag, acl)
	if err != nil {
		return
	}
	fmt.Printf("Created %s\n", path)
	return
}

func (c *Cmd) set(conn *zk.Conn) (err error) {
	path := "/"
	data := ""
	options := c.Options
	if len(options) > 0 {
		path = options[0]
		if len(options) > 1 {
			data = options[1]
		}
	}
	stat, err := conn.Set(path, []byte(data), -1)
	if err != nil {
		return
	}
	fmt.Printf("%s\n", fmtStat(stat))
	return
}

func (c *Cmd) delete(conn *zk.Conn) (err error) {
	path := "/"
	options := c.Options
	if len(options) > 0 {
		path = options[0]
	}
	err = conn.Delete(path, -1)
	if err != nil {
		return
	}
	fmt.Printf("Deleted %s\n", path)
	return
}

func (c *Cmd) run() (err error) {
	switch c.Name {
	case "ls":
		return c.ls(c.Conn)
	case "get":
		return c.get(c.Conn)
	case "create":
		return c.create(c.Conn)
	case "set":
		return c.set(c.Conn)
	case "delete":
		return c.delete(c.Conn)
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
	fmt.Println(`get path
ls path
create path data acl
set path data [version]
delete path [version]
quit
close
connect host:port
addauth scheme auth`)
}

func printRunError(err error) {
	fmt.Println(err)
}

func GetExecutor(conn *zk.Conn) func(s string) {
	return func(s string) {
		c := ParseCmd(s)
		c.Conn = conn
		if c.Name == "quit" || c.Name == "exit" {
			os.Exit(0)
		}
		c.Run()
	}
}
