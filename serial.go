package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	log "github.com/sirupsen/logrus"
	"go.bug.st/serial"
	"sync"
	"time"
)

func insertDB(db *sql.DB, msg string) (int64, error) {
	res, err := db.Exec("INSERT INTO datalog (msg) VALUES (?)", msg)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}
func KeepTrying(ctx context.Context, wg *sync.WaitGroup, sleepTime time.Duration, f func(context context.Context) error) {
	defer wg.Done()
	childContext, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := f(childContext)
			if err != nil {
				log.Error(err)
				time.Sleep(sleepTime)
			}
		}
	}
}

func SerialStream(ctx context.Context, port serial.Port) chan string {
	scanner := bufio.NewScanner(port)
	ch := make(chan string, 1024)
	go func() {
		for scanner.Scan() {
			text := scanner.Text()
			ch <- text
		}
		if scanner.Err() != nil {
			log.Errorf("Failed to read from the serial port: %v\n", scanner.Err())
		}
		close(ch)
	}()
	return ch
}

func ReadSerial(ctx context.Context, address string, baudrate int, db *sql.DB) error {
	//given what pim said I think we should do firmata

	port, err := serial.Open(address, &serial.Mode{
		BaudRate: baudrate,
	})
	if err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	defer func() {
		err := port.Close()
		log.Info("Serial port closed")
		if err != nil {
			log.Errorf("Failed to close the serial port: %v\n", err)
		}
	}()
	serialStream := SerialStream(ctx, port)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-serialStream:
			if !ok {
				return errors.New("serial port is closed")
			}
			id, err := insertDB(db, msg)
			if err != nil {
				return err
			}
			log.Infof("ID: %d, Time: %s, Msg: %s\n", id, time.Now().Format("2006-01-02 15:04:05.000"), msg)
		}
	}
}
