package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"flag"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"time"
)

const create string = `
  CREATE TABLE IF NOT EXISTS datalog (
  id INTEGER NOT NULL PRIMARY KEY,
  timestamp datetime NOT NULL default (STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW', 'localtime')),
  msg TEXT NOT NULL
  );
	CREATE INDEX IF NOT EXISTS datalog_timestamp ON datalog (timestamp);
`

type Arg struct {
	DeviceAddress    string
	Baudrate         int
	DatabaseFileName string
}

func parseArg() (Arg, error) {
	ret := Arg{}
	flag.IntVar(&ret.Baudrate, "baudrate", 9600, "Baudrate of the device")
	flag.StringVar(&ret.DeviceAddress, "device", "", "Device address")
	flag.StringVar(&ret.DatabaseFileName, "db", "data.db", "Database file name")
	flag.Parse()

	if ret.DeviceAddress == "" {
		return ret, errors.New("device address is required")
	}
	return ret, nil
}

func openDB(sourceName string) (*sql.DB, error) {
	// Create the database
	db, err := sql.Open("sqlite3", sourceName)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(create)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	log.SetLevel(log.DebugLevel)
	arg, err := parseArg()
	if err != nil {
		log.Fatal(err)
	}
	db, err := openDB(arg.DatabaseFileName)
	defer func() {
		err := db.Close()
		if err != nil {
			log.Fatalf("Failed to close the database: %v\n", err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go KeepTrying(ctx, wg, 500*time.Millisecond,
		func(ctx context.Context) error {
			return ReadSerial(ctx, arg.DeviceAddress, arg.Baudrate, db)
		})
	go RunWebServer(db)
	for {
		select {
		case <-sigint:
			log.Info("Interrupt signal received, exiting")
			cancel()
			<-ctx.Done()
			wg.Wait()
			log.Info("Done")
			return
		}
	}
}
