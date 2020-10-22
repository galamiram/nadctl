package nadapi

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Direction -
type Direction int

const (
	defaultPort  = "30001"
	maxBrigtness = 3
	// DirectionUp -
	DirectionUp Direction = 1
	// DirectionDown -
	DirectionDown Direction = -1
)

var sources = []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}

// Device is a generic nad receiver
type Device struct {
	ip   net.IP
	port string
	conn net.Conn
}

// New - create a new device object with an open connection
func New(addr, port string) (*Device, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, errors.New("failed to parse ip address")
	}
	if port == "" {
		port = defaultPort
	}
	d := &Device{
		ip:   ip,
		port: port,
	}
	c, err := d.newConn()
	if err != nil {
		return nil, err
	}
	d.conn = c
	return d, nil
}

// PowerOn powers on the device
func (d *Device) PowerOn() error {
	if _, err := d.send("Main.Power=On"); err != nil {
		return err
	}
	return d.reconnect()
}

// PowerOff powers off the device
func (d *Device) PowerOff() error {
	if _, err := d.send("Main.Power=Off"); err != nil {
		return err
	}
	return d.reconnect()
}

// GetPowerState retrieves the current power state
func (d *Device) GetPowerState() (string, error) {
	res, err := d.send("Main.Power?")
	if err != nil {
		return "", fmt.Errorf("get power state: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get power state: %v", err)
	}
	return val, nil
}

// PowerToggle power on/off
func (d *Device) PowerToggle() error {
	state, err := d.GetPowerState()
	if err != nil {
		return err
	}
	if state == "On" {
		return d.PowerOff()
	}
	return d.PowerOn()
}

// GetSource retrieves the current source
func (d *Device) GetSource() (string, error) {
	res, err := d.send("Main.Source?")
	if err != nil {
		return "", fmt.Errorf("get source: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get source: %v", err)
	}
	return val, nil
}

// ToggleSource changes the input source
func (d *Device) ToggleSource(direction Direction) (string, error) {
	src, err := d.GetSource()
	if err != nil {
		return "", err
	}
	for i, s := range sources {
		if s == src {
			pos := i + int(direction)
			if pos > len(sources)-1 {
				pos = 0
			}
			if pos < 0 {
				pos = len(sources) - 1
			}
			cmd := fmt.Sprintf("Main.Source=%s", sources[pos])
			return d.send(cmd)
		}
	}
	return "", errors.New("undefined source name")
}

// GetModel retrieves the model of the device
func (d *Device) GetModel() (string, error) {
	res, err := d.send("Main.Model?")
	if err != nil {
		return "", err
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get device model: %v", err)
	}
	return val, nil
}

// TuneVolume increases or decreases the device volume
func (d *Device) TuneVolume(direction Direction) error {
	vol, err := d.GetVolume()
	if err != nil {
		return err
	}
	v, err := strconv.ParseFloat(vol, 64)
	if err != nil {
		return fmt.Errorf("tune volume: %v", err)
	}
	cmd := fmt.Sprintf("Main.Volume=%f", v+float64(direction))
	_, err = d.send(cmd)
	return err
}

// GetVolume retrieves the volume from the device
func (d *Device) GetVolume() (string, error) {
	res, err := d.send("Main.Volume?")
	if err != nil {
		return "", fmt.Errorf("get volume: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get volume: %v", err)
	}
	return val, nil
}

// GetMuteStatus -
func (d *Device) GetMuteStatus() (string, error) {
	res, err := d.send("Main.Mute?")
	if err != nil {
		return "", fmt.Errorf("get mute status: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get mute status: %v", err)
	}
	return val, nil
}

// ToggleMute -
func (d *Device) ToggleMute() error {
	res, err := d.GetMuteStatus()
	if err != nil {
		return fmt.Errorf("get mute: %v", err)
	}
	if res == "Off" {
		_, err = d.send("Main.Mute=On")
		return err
	}
	_, err = d.send("Main.Mute=Off")
	return err
}

// GetBrightness retrieve the brightness level from the device
func (d *Device) GetBrightness() (string, error) {
	res, err := d.send("Main.Brightness?")
	if err != nil {
		return "", fmt.Errorf("get brightness: %s", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get brightness: %v", err)
	}
	return val, nil
}

// ToggleBrightness change the screen brightness of the device
func (d *Device) ToggleBrightness(direction Direction) error {
	val, err := d.GetBrightness()
	if err != nil {
		return err
	}
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("toggle brightness: %s ", err)
	}
	brightness := intVal + int(direction)
	if brightness > maxBrigtness {
		brightness = 0
	}
	if brightness < 0 {
		brightness = maxBrigtness
	}
	cmd := fmt.Sprintf("Main.Brightness=%d", brightness)
	_, err = d.send(cmd)
	return err
}

// GetRead return bufio reader for reading device messages
func (d *Device) GetRead() (*bufio.Reader, error) {
	c, err := d.newConn()
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(c)
	return r, nil
}

// Disconnect from device
func (d *Device) Disconnect() error {
	return d.conn.Close()
}

func (d *Device) reconnect() error {
	if err := d.Disconnect(); err != nil {
		return err
	}
	c, err := d.newConn()
	if err != nil {
		return err
	}
	d.conn = c
	return nil
}

func (d *Device) newConn() (net.Conn, error) {
	connString := fmt.Sprintf("%s:%s", d.ip.String(), d.port)
	return net.Dial("tcp", connString)
}
func (d *Device) send(cmd string) (string, error) {
	go fmt.Fprintf(d.conn, cmd)
	fmt.Println("b", time.Now())
	status, err := bufio.NewReader(d.conn).ReadString('\n')

	fmt.Println("a", status, time.Now())
	return status, err
}

func trimSuffix(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size-1]
}

func extractValue(raw string) (string, error) {
	s := strings.Split(raw, "=")
	if len(s) > 1 {
		return trimSuffix(s[1]), nil
	}
	return "", errors.New("failed to extract value")
}
