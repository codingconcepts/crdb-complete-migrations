package main

import (
	"complete_migration/api/pkg/repo"
	"complete_migration/api/pkg/server"
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/sijms/go-ora/v2"
)

func main() {
	driver := flag.String("driver", "oracle", "database driver [mysql, oracle, pgx]")
	url := flag.String("url", "", "database connection string")
	addr := flag.String("addr", "localhost:3000", "address to listen on")
	flag.Parse()

	if *url == "" || *driver == "" {
		flag.Usage()
		os.Exit(2)
	}

	db, err := sql.Open(*driver, *url)
	if err != nil {
		log.Fatalf("error opening database connection: %v", err)
	}
	defer func() {
		if err = db.Close(); err != nil {
			log.Fatalf("error closing database connection: %v", err)
		}
	}()

	mustPing(db)

	var r repo.Repo
	switch *driver {
	case "oracle":
		r = repo.NewOracleRepo(db)
	case "mysql":
		r = repo.NewMySQLRepo(db)
	case "pgx":
		r = repo.NewPostgresRepo(db)
	}

	svr := server.New(*addr, r)
	log.Fatal(svr.Start())
}

func mustPing(db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("error pinging database: %v", err)
	}
}
