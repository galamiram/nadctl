package nadapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Direction -
type Direction int

const (
	defaultPort   = "30001"
	maxBrightness = 3
	// DirectionUp -
	DirectionUp Direction = 1
	// DirectionDown -
	DirectionDown Direction = -1
)

var sources = []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}

// Device is a generic nad receiver
type Device struct {
	IP   net.IP
	Port string
	conn net.Conn
}

// DiscoveredDevice represents a NAD device found on the network
type DiscoveredDevice struct {
	IP    string
	Model string
	Port  string
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
		IP:   ip,
		Port: port,
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

// GetAvailableSources returns the list of available input sources
func GetAvailableSources() []string {
	return sources
}

// GetAvailableBrightnessLevels returns the list of available brightness levels
func GetAvailableBrightnessLevels() []int {
	levels := make([]int, maxBrightness+1)
	for i := 0; i <= maxBrightness; i++ {
		levels[i] = i
	}
	return levels
}

// IsValidBrightnessLevel checks if the given brightness level is valid
func IsValidBrightnessLevel(level int) bool {
	return level >= 0 && level <= maxBrightness
}

// IsValidSource checks if the given source name is valid
func IsValidSource(source string) bool {
	for _, s := range sources {
		if strings.EqualFold(s, source) {
			return true
		}
	}
	return false
}

// SetSource sets the input source to a specific source name
func (d *Device) SetSource(sourceName string) error {
	// Validate source name (case-insensitive)
	var validSource string
	for _, s := range sources {
		if strings.EqualFold(s, sourceName) {
			validSource = s
			break
		}
	}

	if validSource == "" {
		return fmt.Errorf("invalid source '%s'. Available sources: %v", sourceName, sources)
	}

	cmd := fmt.Sprintf("Main.Source=%s", validSource)
	_, err := d.send(cmd)
	return err
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

// SetVolume sets the volume to a specific level
func (d *Device) SetVolume(volume float64) error {
	// NAD devices typically support volume ranges, but let's be safe with bounds
	if volume < -80 {
		volume = -80 // Typical minimum volume
	}
	if volume > 10 {
		volume = 10 // Typical maximum volume to prevent damage
	}

	cmd := fmt.Sprintf("Main.Volume=%f", volume)
	_, err := d.send(cmd)
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

// GetVolumeFloat retrieves the volume as a float64 value
func (d *Device) GetVolumeFloat() (float64, error) {
	volStr, err := d.GetVolume()
	if err != nil {
		return 0, err
	}

	vol, err := strconv.ParseFloat(volStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse volume: %v", err)
	}

	return vol, nil
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

// GetBrightnessInt retrieves the brightness level as an integer
func (d *Device) GetBrightnessInt() (int, error) {
	brightnessStr, err := d.GetBrightness()
	if err != nil {
		return 0, err
	}

	brightness, err := strconv.Atoi(brightnessStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse brightness: %v", err)
	}

	return brightness, nil
}

// SetBrightness sets the brightness to a specific level (0-3)
func (d *Device) SetBrightness(level int) error {
	if !IsValidBrightnessLevel(level) {
		return fmt.Errorf("invalid brightness level %d. Valid levels: %v", level, GetAvailableBrightnessLevels())
	}

	cmd := fmt.Sprintf("Main.Brightness=%d", level)
	_, err := d.send(cmd)
	return err
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
	if brightness > maxBrightness {
		brightness = 0
	}
	if brightness < 0 {
		brightness = maxBrightness
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
	connString := fmt.Sprintf("%s:%s", d.IP.String(), d.Port)
	return net.Dial("tcp", connString)
}
func (d *Device) send(cmd string) (string, error) {
	_, err := fmt.Fprintf(d.conn, cmd)
	if err != nil {
		return "", err
	}

	status, err := bufio.NewReader(d.conn).ReadString('\n')
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

// DiscoverDevices scans the local network for NAD devices
func DiscoverDevices(timeout time.Duration) ([]DiscoveredDevice, error) {
	// Get local network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	var devices []DiscoveredDevice
	var wg sync.WaitGroup
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Scan each network interface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}

			// Scan this subnet
			wg.Add(1)
			go func(subnet *net.IPNet) {
				defer wg.Done()
				found := scanSubnet(ctx, subnet)
				mu.Lock()
				devices = append(devices, found...)
				mu.Unlock()
			}(ipNet)
		}
	}

	wg.Wait()
	return devices, nil
}

// scanSubnet scans a subnet for NAD devices
func scanSubnet(ctx context.Context, subnet *net.IPNet) []DiscoveredDevice {
	var devices []DiscoveredDevice
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Generate IPs in the subnet
	for ip := subnet.IP.Mask(subnet.Mask); subnet.Contains(ip); inc(ip) {
		if ctx.Err() != nil {
			break
		}

		ipStr := ip.String()
		// Skip network and broadcast addresses
		if ipStr == subnet.IP.String() || strings.HasSuffix(ipStr, ".0") || strings.HasSuffix(ipStr, ".255") {
			continue
		}

		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			if device := testNADDevice(ctx, target); device != nil {
				mu.Lock()
				devices = append(devices, *device)
				mu.Unlock()
			}
		}(ipStr)
	}

	wg.Wait()
	return devices
}

// testNADDevice tests if an IP address hosts a NAD device
func testNADDevice(ctx context.Context, ip string) *DiscoveredDevice {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, defaultPort), 2*time.Second)
	if err != nil {
		return nil
	}
	defer conn.Close()

	// Set deadline for the connection
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	}

	// Try to get the model to verify it's a NAD device
	_, err = fmt.Fprintf(conn, "Main.Model?")
	if err != nil {
		return nil
	}

	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil
	}

	// Extract model from response
	model, err := extractValue(response)
	if err != nil {
		return nil
	}

	// Verify it looks like a NAD model
	if strings.Contains(strings.ToUpper(model), "NAD") ||
		strings.Contains(strings.ToUpper(model), "C338") {
		return &DiscoveredDevice{
			IP:    ip,
			Model: model,
			Port:  defaultPort,
		}
	}

	return nil
}

// inc increments an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
