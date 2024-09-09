## LabWatch

Simple Serial to DB monitoring. Intended to be run 24/7 for small computer like raspberry pi.

## Features

- Serial port monitoring
- Database logging
- Web interface with csv filtered export
- Hot Plug Support. (Reconnect to serial port if disconnected)

## Requirements

Arduino or something that print data(number) to serial port.

## Development

### Compile

```go build -o labwatch *.go```

### Run

```./labwatch --device /dev/ttyUSB0 --baudrate 9600```

**Tips**

- Listing port on linux system ```ls /dev/tty*``` or ```lsusb```
- On OSX it looks like ```/dev/cu.usbmodem1301```

### Run on startup in linux

There is an example of init.d in ```init.d.example file``` edit and copy it to /etc/init.d then use update-rc.d to add
it to startup.
