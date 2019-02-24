package bench

import (
	"os"
	"fmt"
	"flag"
	"sort"
	"sync"
	"time"
	"bufio"
	"context"
	"io/ioutil"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/subcommands"
	"github.com/lucklove/mysql-replayer/utils"
)

type QueryTask struct {
	ts int64
	sql string
}

type FileInfoSortByName []os.FileInfo

func (s FileInfoSortByName) Len() int {
    return len(s)
}
func (s FileInfoSortByName) Swap(i, j int) {
    s[i], s[j] = s[j], s[i]
}
func (s FileInfoSortByName) Less(i, j int) bool {
    return s[i].Name() < s[j].Name()
}

type BenchCommand struct {
	input string
	host string
	port string
	user string
	passwd string
	speed int
	concurrent int
}

func (*BenchCommand) Name() string     { return "bench" }
func (*BenchCommand) Synopsis() string { return "Bench mysql server." }
func (*BenchCommand) Usage() string {
	return `bench -i input-dir [-h host] [-P port] [-u user] [-p passwd] [-s speed] [-c concurrent]:
	Bench mysql server with data from input-dir.
`
}
  
func (b *BenchCommand) SetFlags(f *flag.FlagSet) {
	f.StringVar(&b.input, "i", "", "the directory contains bench data")
	f.StringVar(&b.host, "h", "127.0.0.1", "connect to host")
	f.StringVar(&b.port, "P", "4000", "port number to use for connection")
	f.StringVar(&b.user, "u", "root", "user for login")
	f.StringVar(&b.passwd, "p", "", "password to use when connecting to server")
	f.IntVar(&b.speed, "s", 1, "the bench speed, default 1")
	f.IntVar(&b.concurrent, "c", 1, "the bench concurrent, default 1")
}
  
func (b *BenchCommand) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(b.input) == 0 || len(b.host) == 0 || len(b.port) == 0 || len(b.user) == 0 {
		fmt.Println(b.Usage())
		return subcommands.ExitSuccess
	}

	fch := make(chan string, 10)			// Pass file name
	cch := make(chan int64, b.concurrent)	// Collect query count
	wg := sync.WaitGroup{}
	start := int64(time.Now().Unix())

	fmt.Println("Processing...")
	wg.Add(b.concurrent)
	for i := 0; i < b.concurrent; i++ {
		go func() {
			defer wg.Done()
			cch <- b.bench(fch)
		}()
	}

	if files, err := ioutil.ReadDir(b.input); err == nil {
		fmt.Println(len(files))
		sort.Sort(FileInfoSortByName(files))

		for _, f := range files {
			fch <- f.Name()
		}

		close(fch)
	}

	wg.Wait()
	end := int64(time.Now().Unix())

	var count int64 = 0
	for i := 0; i < b.concurrent; i++ {
		count += <- cch
	}

	fmt.Printf("Process %d querys in %d seconds, QPS: %d\n", count, end - start, count / (end - start))

	return subcommands.ExitSuccess
}

func (b *BenchCommand) bench(ch chan string) int64 {
	var count int64 = 0

	for name := range ch {
		var synts int64 = 0
		fmt.Sscanf(name, "%d", &synts)

		path := fmt.Sprintf("%s/%s", b.input, name)
		if file, err := os.Open(path); err == nil {
			reader := bufio.NewReader(file)

			dbname, err := reader.ReadString('\n')
			if err != nil {
				utils.LogIOError(err)
				file.Close()
				continue
			}
			fmt.Sscanf(dbname, "%s\n", &dbname)

			tch := make(chan *QueryTask, 100)
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				b.benchWoker(dbname, synts, tch)
			}()
			
			loop:
			for {
				task := new(QueryTask)

				line, err := reader.ReadString('\n')
				if err != nil {
					utils.LogIOError(err)
					break loop
				}
				fmt.Sscanf(line, "%d", &task.ts)

				sqllen := 0
				line, err = reader.ReadString('\n')
				if err != nil {
					utils.LogIOError(err)
					break loop
				}
				fmt.Sscanf(line, "%d\n", &sqllen)

				for len(task.sql) < sqllen {
					line, err = reader.ReadString('\n')
					if err != nil {
						utils.LogIOError(err)
						break loop
					}

					task.sql += line
				}
				task.sql = task.sql[:len(task.sql)-1]	// Trim last '\n'

				tch <- task
				count++
			}

			close(tch)
			file.Close()
			wg.Wait()
		}
	}

	return count
}

func (b *BenchCommand) benchWoker(dbname string, synts int64, ch chan *QueryTask) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", b.user, b.passwd, b.host, b.port, dbname)
	startts := int64(time.Now().Unix())

	if db, err := sql.Open("mysql", dsn); err == nil {
		defer db.Close()

		for task := range ch {
			curts := int64(time.Now().Unix())
			delta := (task.ts - synts) / int64(b.speed) - (curts - startts)

			if delta > 0 {
				time.Sleep(time.Duration(delta) * time.Second)
			}

			db.Exec(task.sql)
		}
	}
}