package tools

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"sync"
)

type Status int

const (
	InsertBlog = `
		INSERT INTO Blog (title, status, createDate, updateDate)
		VALUES (?, ?, ?, ?)`

	InsertTarget = `
		INSERT INTO blog_target(blog_id, target)
		VALUES (?, ?)
	`
)

const (
	Draft Status = iota
	Published
	Deleted
)

func (s Status) String() string {
	switch s {
	case Draft:
		return "Draft"
	case Published:
		return "Published"
	case Deleted:
		return "Deleted"
	default:
		return "Undefined"
	}
}

type Database struct { // 单例模式存储sql连接
	conn *sql.DB
}

var (
	instance *Database
	once     sync.Once // 并发原语，用于在程序运行期间只执行一次某个操作。
)

func GetConnection() *Database {
	once.Do(func() { // Do方法使用了互斥锁和原子操作来确保传入的函数只被执行一次
		instance = &Database{}
		db, err := sql.Open("sqlite3", "./resource/EasyBlog.db")
		if err != nil {
			log.Fatal(err)
		}
		instance.conn = db
	})

	return instance
}

func (db *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

func (db *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

func (db *Database) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

func (db *Database) Close() {
	_ = db.conn.Close()
}
