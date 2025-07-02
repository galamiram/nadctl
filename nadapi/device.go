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

	log "github.com/sirupsen/logrus"
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
	log.WithFields(log.Fields{
		"address": addr,
		"port":    port,
	}).Debug("Creating new NAD device connection")

	ip := net.ParseIP(addr)
	if ip == nil {
		log.WithField("address", addr).Debug("Failed to parse IP address")
		return nil, errors.New("failed to parse ip address")
	}
	if port == "" {
		port = defaultPort
		log.WithField("port", port).Debug("Using default port")
	}
	d := &Device{
		IP:   ip,
		Port: port,
	}

	log.WithFields(log.Fields{
		"ip":   d.IP.String(),
		"port": d.Port,
	}).Debug("Attempting to establish connection to NAD device")

	c, err := d.newConn()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"ip":   d.IP.String(),
			"port": d.Port,
		}).Debug("Failed to establish connection")
		return nil, err
	}
	d.conn = c

	log.WithFields(log.Fields{
		"ip":   d.IP.String(),
		"port": d.Port,
	}).Debug("Successfully connected to NAD device")

	return d, nil
}

// PowerOn powers on the device
func (d *Device) PowerOn() error {
	log.WithField("device", d.IP.String()).Debug("Powering on device")
	if _, err := d.send("Main.Power=On"); err != nil {
		return err
	}
	return d.reconnect()
}

// PowerOff powers off the device
func (d *Device) PowerOff() error {
	log.WithField("device", d.IP.String()).Debug("Powering off device")
	if _, err := d.send("Main.Power=Off"); err != nil {
		return err
	}
	return d.reconnect()
}

// GetPowerState retrieves the current power state
func (d *Device) GetPowerState() (string, error) {
	log.WithField("device", d.IP.String()).Debug("Getting power state")
	res, err := d.send("Main.Power?")
	if err != nil {
		return "", fmt.Errorf("get power state: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get power state: %v", err)
	}
	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"state":  val,
	}).Debug("Retrieved power state")
	return val, nil
}

// PowerToggle power on/off
func (d *Device) PowerToggle() error {
	log.WithField("device", d.IP.String()).Debug("Toggling power state")
	state, err := d.GetPowerState()
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"device":       d.IP.String(),
		"currentState": state,
	}).Debug("Current power state retrieved for toggle")

	if state == "On" {
		return d.PowerOff()
	}
	return d.PowerOn()
}

// GetSource retrieves the current source
func (d *Device) GetSource() (string, error) {
	log.WithField("device", d.IP.String()).Debug("Getting current source")
	res, err := d.send("Main.Source?")
	if err != nil {
		return "", fmt.Errorf("get source: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get source: %v", err)
	}
	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"source": val,
	}).Debug("Retrieved current source")
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
	log.WithFields(log.Fields{
		"device":     d.IP.String(),
		"sourceName": sourceName,
	}).Debug("Setting source")

	// Validate source name (case-insensitive)
	var validSource string
	for _, s := range sources {
		if strings.EqualFold(s, sourceName) {
			validSource = s
			break
		}
	}

	if validSource == "" {
		log.WithFields(log.Fields{
			"device":           d.IP.String(),
			"invalidSource":    sourceName,
			"availableSources": sources,
		}).Debug("Invalid source name provided")
		return fmt.Errorf("invalid source '%s'. Available sources: %v", sourceName, sources)
	}

	log.WithFields(log.Fields{
		"device":      d.IP.String(),
		"sourceName":  sourceName,
		"validSource": validSource,
	}).Debug("Validated source name")

	cmd := fmt.Sprintf("Main.Source=%s", validSource)
	_, err := d.send(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": d.IP.String(),
			"source": validSource,
		}).Debug("Failed to set source")
		return err
	}

	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"source": validSource,
	}).Debug("Successfully set source")
	return nil
}

// ToggleSource changes the input source
func (d *Device) ToggleSource(direction Direction) (string, error) {
	log.WithFields(log.Fields{
		"device":    d.IP.String(),
		"direction": direction,
	}).Debug("Toggling source")

	src, err := d.GetSource()
	if err != nil {
		return "", err
	}

	log.WithFields(log.Fields{
		"device":        d.IP.String(),
		"currentSource": src,
		"direction":     direction,
	}).Debug("Current source retrieved for toggle")

	for i, s := range sources {
		if s == src {
			pos := i + int(direction)
			if pos > len(sources)-1 {
				pos = 0
			}
			if pos < 0 {
				pos = len(sources) - 1
			}
			newSource := sources[pos]

			log.WithFields(log.Fields{
				"device":   d.IP.String(),
				"from":     src,
				"to":       newSource,
				"position": pos,
			}).Debug("Calculated new source position")

			cmd := fmt.Sprintf("Main.Source=%s", newSource)
			return d.send(cmd)
		}
	}
	return "", errors.New("undefined source name")
}

// GetModel retrieves the model of the device
func (d *Device) GetModel() (string, error) {
	log.WithField("device", d.IP.String()).Debug("Getting device model")
	res, err := d.send("Main.Model?")
	if err != nil {
		return "", err
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get device model: %v", err)
	}
	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"model":  val,
	}).Debug("Retrieved device model")
	return val, nil
}

// TuneVolume increases or decreases the device volume
func (d *Device) TuneVolume(direction Direction) error {
	log.WithFields(log.Fields{
		"device":    d.IP.String(),
		"direction": direction,
	}).Debug("Tuning volume")

	vol, err := d.GetVolume()
	if err != nil {
		return err
	}
	v, err := strconv.ParseFloat(vol, 64)
	if err != nil {
		return fmt.Errorf("tune volume: %v", err)
	}

	newVolume := v + float64(direction)
	log.WithFields(log.Fields{
		"device":     d.IP.String(),
		"currentVol": v,
		"direction":  direction,
		"newVolume":  newVolume,
	}).Debug("Calculated new volume level")

	cmd := fmt.Sprintf("Main.Volume=%f", newVolume)
	_, err = d.send(cmd)
	return err
}

// SetVolume sets the volume to a specific level
func (d *Device) SetVolume(volume float64) error {
	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"volume": volume,
	}).Debug("Setting volume")

	// NAD devices typically support volume ranges, but let's be safe with bounds
	originalVolume := volume
	if volume < -80 {
		volume = -80 // Typical minimum volume
		log.WithFields(log.Fields{
			"device":       d.IP.String(),
			"requestedVol": originalVolume,
			"adjustedVol":  volume,
		}).Debug("Volume clamped to minimum (-80 dB)")
	}
	if volume > 10 {
		volume = 10 // Typical maximum volume to prevent damage
		log.WithFields(log.Fields{
			"device":       d.IP.String(),
			"requestedVol": originalVolume,
			"adjustedVol":  volume,
		}).Debug("Volume clamped to maximum (10 dB)")
	}

	cmd := fmt.Sprintf("Main.Volume=%f", volume)
	_, err := d.send(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": d.IP.String(),
			"volume": volume,
		}).Debug("Failed to set volume")
	} else {
		log.WithFields(log.Fields{
			"device": d.IP.String(),
			"volume": volume,
		}).Debug("Successfully set volume")
	}
	return err
}

// GetVolume retrieves the volume from the device
func (d *Device) GetVolume() (string, error) {
	log.WithField("device", d.IP.String()).Debug("Getting volume")
	res, err := d.send("Main.Volume?")
	if err != nil {
		return "", fmt.Errorf("get volume: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get volume: %v", err)
	}
	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"volume": val,
	}).Debug("Retrieved volume")
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
		log.WithError(err).WithFields(log.Fields{
			"device":    d.IP.String(),
			"volumeStr": volStr,
		}).Debug("Failed to parse volume as float")
		return 0, fmt.Errorf("failed to parse volume: %v", err)
	}

	log.WithFields(log.Fields{
		"device":      d.IP.String(),
		"volumeFloat": vol,
	}).Debug("Retrieved volume as float")
	return vol, nil
}

// GetMuteStatus -
func (d *Device) GetMuteStatus() (string, error) {
	log.WithField("device", d.IP.String()).Debug("Getting mute status")
	res, err := d.send("Main.Mute?")
	if err != nil {
		return "", fmt.Errorf("get mute status: %v", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get mute status: %v", err)
	}
	log.WithFields(log.Fields{
		"device":     d.IP.String(),
		"muteStatus": val,
	}).Debug("Retrieved mute status")
	return val, nil
}

// ToggleMute -
func (d *Device) ToggleMute() error {
	log.WithField("device", d.IP.String()).Debug("Toggling mute")
	res, err := d.GetMuteStatus()
	if err != nil {
		return fmt.Errorf("get mute: %v", err)
	}

	log.WithFields(log.Fields{
		"device":      d.IP.String(),
		"currentMute": res,
	}).Debug("Current mute status retrieved for toggle")

	if res == "Off" {
		log.WithField("device", d.IP.String()).Debug("Muting device (turning mute On)")
		_, err = d.send("Main.Mute=On")
		return err
	}
	log.WithField("device", d.IP.String()).Debug("Unmuting device (turning mute Off)")
	_, err = d.send("Main.Mute=Off")
	return err
}

// GetBrightness retrieve the brightness level from the device
func (d *Device) GetBrightness() (string, error) {
	log.WithField("device", d.IP.String()).Debug("Getting brightness")
	res, err := d.send("Main.Brightness?")
	if err != nil {
		return "", fmt.Errorf("get brightness: %s", err)
	}
	val, err := extractValue(res)
	if err != nil {
		return "", fmt.Errorf("get brightness: %v", err)
	}
	log.WithFields(log.Fields{
		"device":     d.IP.String(),
		"brightness": val,
	}).Debug("Retrieved brightness")
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
		log.WithError(err).WithFields(log.Fields{
			"device":        d.IP.String(),
			"brightnessStr": brightnessStr,
		}).Debug("Failed to parse brightness as integer")
		return 0, fmt.Errorf("failed to parse brightness: %v", err)
	}

	log.WithFields(log.Fields{
		"device":        d.IP.String(),
		"brightnessInt": brightness,
	}).Debug("Retrieved brightness as integer")
	return brightness, nil
}

// SetBrightness sets the brightness to a specific level (0-3)
func (d *Device) SetBrightness(level int) error {
	log.WithFields(log.Fields{
		"device": d.IP.String(),
		"level":  level,
	}).Debug("Setting brightness")

	if !IsValidBrightnessLevel(level) {
		log.WithFields(log.Fields{
			"device":       d.IP.String(),
			"invalidLevel": level,
			"validLevels":  GetAvailableBrightnessLevels(),
		}).Debug("Invalid brightness level provided")
		return fmt.Errorf("invalid brightness level %d. Valid levels: %v", level, GetAvailableBrightnessLevels())
	}

	cmd := fmt.Sprintf("Main.Brightness=%d", level)
	_, err := d.send(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": d.IP.String(),
			"level":  level,
		}).Debug("Failed to set brightness")
	} else {
		log.WithFields(log.Fields{
			"device": d.IP.String(),
			"level":  level,
		}).Debug("Successfully set brightness")
	}
	return err
}

// ToggleBrightness change the screen brightness of the device
func (d *Device) ToggleBrightness(direction Direction) error {
	log.WithFields(log.Fields{
		"device":    d.IP.String(),
		"direction": direction,
	}).Debug("Toggling brightness")

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

	log.WithFields(log.Fields{
		"device":       d.IP.String(),
		"currentLevel": intVal,
		"direction":    direction,
		"newLevel":     brightness,
	}).Debug("Calculated new brightness level")

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
	log.WithField("device", d.IP.String()).Debug("Reconnecting to device")
	if err := d.Disconnect(); err != nil {
		log.WithError(err).WithField("device", d.IP.String()).Debug("Error during disconnect for reconnect")
		return err
	}
	c, err := d.newConn()
	if err != nil {
		log.WithError(err).WithField("device", d.IP.String()).Debug("Failed to establish new connection during reconnect")
		return err
	}
	d.conn = c
	log.WithField("device", d.IP.String()).Debug("Successfully reconnected")
	return nil
}

func (d *Device) newConn() (net.Conn, error) {
	connString := fmt.Sprintf("%s:%s", d.IP.String(), d.Port)
	log.WithFields(log.Fields{
		"device":     d.IP.String(),
		"connString": connString,
	}).Debug("Creating new TCP connection")

	conn, err := net.Dial("tcp", connString)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device":     d.IP.String(),
			"connString": connString,
		}).Debug("Failed to create TCP connection")
	} else {
		log.WithFields(log.Fields{
			"device":     d.IP.String(),
			"connString": connString,
		}).Debug("Successfully created TCP connection")
	}
	return conn, err
}

func (d *Device) send(cmd string) (string, error) {
	log.WithFields(log.Fields{
		"device":  d.IP.String(),
		"command": cmd,
	}).Debug("Sending command to device")

	_, err := fmt.Fprintf(d.conn, "%s", cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device":  d.IP.String(),
			"command": cmd,
		}).Debug("Failed to send command")
		return "", err
	}

	status, err := bufio.NewReader(d.conn).ReadString('\n')
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device":  d.IP.String(),
			"command": cmd,
		}).Debug("Failed to read response")
		return "", err
	}

	log.WithFields(log.Fields{
		"device":   d.IP.String(),
		"command":  cmd,
		"response": strings.TrimSpace(status),
	}).Debug("Received response from device")

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
	log.WithField("timeout", timeout).Debug("Starting NAD device discovery")

	// Get local network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		log.WithError(err).Debug("Failed to get network interfaces")
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	log.WithField("interfaceCount", len(interfaces)).Debug("Retrieved network interfaces")

	var devices []DiscoveredDevice
	var wg sync.WaitGroup
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	scannedSubnets := 0
	// Scan each network interface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			log.WithFields(log.Fields{
				"interface": iface.Name,
				"flags":     iface.Flags,
			}).Debug("Skipping interface (down or loopback)")
			continue
		}

		log.WithField("interface", iface.Name).Debug("Processing network interface")

		addrs, err := iface.Addrs()
		if err != nil {
			log.WithError(err).WithField("interface", iface.Name).Debug("Failed to get interface addresses")
			continue
		}

		log.WithFields(log.Fields{
			"interface":    iface.Name,
			"addressCount": len(addrs),
		}).Debug("Retrieved interface addresses")

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				log.WithFields(log.Fields{
					"interface": iface.Name,
					"address":   addr.String(),
				}).Debug("Skipping non-IPv4 address")
				continue
			}

			log.WithFields(log.Fields{
				"interface": iface.Name,
				"subnet":    ipNet.String(),
			}).Debug("Scanning subnet for NAD devices")

			scannedSubnets++
			// Scan this subnet
			wg.Add(1)
			go func(subnet *net.IPNet, ifaceName string) {
				defer wg.Done()
				found := scanSubnet(ctx, subnet)
				mu.Lock()
				devices = append(devices, found...)
				log.WithFields(log.Fields{
					"interface":    ifaceName,
					"subnet":       subnet.String(),
					"devicesFound": len(found),
				}).Debug("Subnet scan completed")
				mu.Unlock()
			}(ipNet, iface.Name)
		}
	}

	log.WithField("scannedSubnets", scannedSubnets).Debug("Waiting for all subnet scans to complete")
	wg.Wait()

	log.WithFields(log.Fields{
		"totalDevices": len(devices),
		"timeout":      timeout,
	}).Debug("Device discovery completed")

	return devices, nil
}

// scanSubnet scans a subnet for NAD devices
func scanSubnet(ctx context.Context, subnet *net.IPNet) []DiscoveredDevice {
	log.WithField("subnet", subnet.String()).Debug("Starting subnet scan")

	var devices []DiscoveredDevice
	var wg sync.WaitGroup
	var mu sync.Mutex

	scannedIPs := 0
	foundDevices := 0

	// Generate IPs in the subnet
	for ip := subnet.IP.Mask(subnet.Mask); subnet.Contains(ip); inc(ip) {
		if ctx.Err() != nil {
			log.WithFields(log.Fields{
				"subnet": subnet.String(),
				"error":  ctx.Err(),
			}).Debug("Subnet scan cancelled due to context")
			break
		}

		ipStr := ip.String()
		// Skip network and broadcast addresses
		if ipStr == subnet.IP.String() || strings.HasSuffix(ipStr, ".0") || strings.HasSuffix(ipStr, ".255") {
			log.WithFields(log.Fields{
				"subnet": subnet.String(),
				"ip":     ipStr,
			}).Debug("Skipping network/broadcast address")
			continue
		}

		scannedIPs++
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			if device := testNADDevice(ctx, target); device != nil {
				mu.Lock()
				devices = append(devices, *device)
				foundDevices++
				log.WithFields(log.Fields{
					"subnet": subnet.String(),
					"ip":     target,
					"model":  device.Model,
				}).Debug("Found NAD device")
				mu.Unlock()
			}
		}(ipStr)
	}

	log.WithFields(log.Fields{
		"subnet":     subnet.String(),
		"scannedIPs": scannedIPs,
	}).Debug("Waiting for IP scans to complete in subnet")

	wg.Wait()

	log.WithFields(log.Fields{
		"subnet":       subnet.String(),
		"scannedIPs":   scannedIPs,
		"foundDevices": foundDevices,
	}).Debug("Subnet scan completed")

	return devices
}

// testNADDevice tests if an IP address hosts a NAD device
func testNADDevice(ctx context.Context, ip string) *DiscoveredDevice {
	log.WithField("ip", ip).Debug("Testing IP for NAD device")

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, defaultPort), 2*time.Second)
	if err != nil {
		log.WithFields(log.Fields{
			"ip":    ip,
			"error": err.Error(),
		}).Debug("Failed to connect to IP")
		return nil
	}
	defer conn.Close()

	log.WithField("ip", ip).Debug("Successfully connected, testing for NAD device")

	// Set deadline for the connection
	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	}

	// Try to get the model to verify it's a NAD device
	_, err = fmt.Fprintf(conn, "Main.Model?")
	if err != nil {
		log.WithFields(log.Fields{
			"ip":      ip,
			"command": "Main.Model?",
			"error":   err.Error(),
		}).Debug("Failed to send model query")
		return nil
	}

	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.WithFields(log.Fields{
			"ip":      ip,
			"command": "Main.Model?",
			"error":   err.Error(),
		}).Debug("Failed to read model response")
		return nil
	}

	log.WithFields(log.Fields{
		"ip":       ip,
		"response": strings.TrimSpace(response),
	}).Debug("Received model response")

	// Extract model from response
	model, err := extractValue(response)
	if err != nil {
		log.WithFields(log.Fields{
			"ip":       ip,
			"response": strings.TrimSpace(response),
			"error":    err.Error(),
		}).Debug("Failed to extract model from response")
		return nil
	}

	log.WithFields(log.Fields{
		"ip":    ip,
		"model": model,
	}).Debug("Extracted model from response")

	// Verify it looks like a NAD model
	if strings.Contains(strings.ToUpper(model), "NAD") ||
		strings.Contains(strings.ToUpper(model), "C338") {
		log.WithFields(log.Fields{
			"ip":    ip,
			"model": model,
		}).Debug("Confirmed NAD device")
		return &DiscoveredDevice{
			IP:    ip,
			Model: model,
			Port:  defaultPort,
		}
	}

	log.WithFields(log.Fields{
		"ip":    ip,
		"model": model,
	}).Debug("Device model does not appear to be NAD")

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
