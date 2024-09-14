package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Error struct {
	errs    []error
	field   string
	comment string
}

func (err *Error) Error() string {
	s := ""
	if err.field != "" {
		s = "field: " + err.field + " --"
	}
	for _, err := range err.errs {
		if err != nil {
			if s != "" {
				s += "\n"
			}
			s += "\t" + err.Error()
		}
	}
	if err.comment != "" {
		if s != "" {
			s += "\n"
		}
		s += "\t" + err.comment
	}
	return s
}

func Err(field, comment string, errs ...error) *Error {
	return &Error{
		errs:    errs,
		field:   field,
		comment: comment,
	}
}

type Machine int

const (
	ClientMachine = iota
	ReplicaMachine
	MasterMachine
)

var readingClients = false

type Config struct {

	// -- client info --
	// number of client requests
	Reqs int
	// duration during which a client run
	RunTime time.Duration
	// ration of writes
	Writes int
	// conflict ratio
	Conflicts int
	// the size of payload
	CommandSize int
	// number of clones of each client
	Clones int
	// wait reply from the closest replica
	WaitClosest bool
	Pipeline    bool
	// when pipelining the frequency of syncs
	Syncs int
	// when pipelining the maximal number of pending commands
	Pendings int
	// Hot key for this set of clients
	Key int
}

func Read(filename, alias string) (*Config, error) {
	c := &Config{}

	f, err := os.Open(filename)
	if err != nil {
		return c, err
	}
	defer f.Close()

	var (
		apply = true
	)

	s := bufio.NewScanner(f)
	for s.Scan() {
		txt := strings.ToLower(s.Text())
		words := strings.Fields(txt)
		if len(words) < 1 {
			continue
		}
		switch words[0] {
		case "//":
			continue
		case "--":
			if len(words) < 2 {
				return c, Err("", "expecting [Replicas | Clients | Master | Apply | Proxy] after --")
			}
			apply = true
			switch strings.ToLower(words[1]) {
			case "clients":
				readingClients = true
			case "apply":
				if len(words) < 4 || words[2] != "to" {
					return c, Err("-- Apply", "Missing argument")
				}
				if words[3] != alias {
					apply = false
				}
			}
		default:
			if !apply {
				continue
			}
			var (
				ok  = false
				err error
			)
			switch strings.Split(words[0], ":")[0] {

			case "reqs":
				c.Reqs, err = expectInt(words)
				ok = true
			case "writes":
				c.Writes, err = expectInt(words)
				ok = true
			case "conflicts":
				c.Conflicts, err = expectInt(words)
				ok = true
			case "commandSize":
				c.CommandSize, err = expectInt(words)
				ok = true
			case "clones":
				c.Clones, err = expectInt(words)
				ok = true
			case "runtime":
				c.RunTime, err = expectDuration(words)
				ok = true
			case "pipeline":
				c.Pipeline, err = expectBool(words)
				ok = true
			case "pendings":
				c.Pendings, err = expectInt(words)
				ok = true
			case "key":
				c.Key, err = expectInt(words)
				ok = true
			case "commandsize":
				c.CommandSize, err = expectInt(words)
				ok = true
			}
			if ok {
				readingClients = false
				if err != nil {
					return c, err
				}
			}
		}
	}

	return c, nil
}

func expectInt(ws []string) (int, error) {
	return expect(ws, strconv.Atoi, 0)
}

// func expectString(ws []string) (string, error) {
// 	return expect(ws, func(s string) (string, error) {
// 		return s, nil
// 	}, "")
// }

func expectBool(ws []string) (bool, error) {
	return expect(ws, strconv.ParseBool, false)
}

func expectDuration(ws []string) (time.Duration, error) {
	return expect(ws, func(s string) (time.Duration, error) {
		if s == "none" {
			return time.Duration(0), nil
		}
		return time.ParseDuration(s)
	}, time.Duration(0))
}

type expectRet interface {
	int | string | bool | time.Duration
}

func expect[R expectRet](ws []string, f func(string) (R, error), none R) (R, error) {
	if ws == nil || len(ws) < 1 {
		return none, Err("", "Missing field")
	}
	if len(ws) < 2 || strings.HasPrefix(ws[1], "//") {
		return none, Err(ws[0], "Missing argument")
	}
	i, err := f(ws[1])
	if err != nil {
		return i, Err(ws[0], "Invalid argument", err)
	}
	return i, nil
}
