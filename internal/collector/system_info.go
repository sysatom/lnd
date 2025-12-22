package collector

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/vishvananda/netlink"
)

type SystemCollector struct{}

func NewSystemCollector() *SystemCollector {
	return &SystemCollector{}
}

func (c *SystemCollector) Collect() (info HostInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in SystemCollector: %v", r)
			info.Error = err
		}
	}()

	info = HostInfo{
		SysctlParams: make(map[string]string),
	}

	// Host Info
	h, err := host.Info()
	if err == nil {
		info.Hostname = h.Hostname
		info.KernelVersion = h.KernelVersion
		info.Arch = h.KernelArch
		info.Uptime = time.Duration(h.Uptime) * time.Second
	} else {
		info.Error = err
	}

	// Load Avg
	l, err := load.Avg()
	if err == nil {
		info.LoadAvg = l.Load1
	}

	// Resource Limits
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		info.MaxOpenFiles = rLimit.Cur
	}

	// fs.file-max
	if content, err := ioutil.ReadFile("/proc/sys/fs/file-max"); err == nil {
		if val, err := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64); err == nil {
			info.FileMax = val
		}
	}

	// Sysctl Params
	sysctlKeys := []string{
		"net/core/somaxconn",
		"net/ipv4/tcp_tw_reuse",
		"net/ipv4/ip_local_port_range",
	}
	for _, key := range sysctlKeys {
		if content, err := ioutil.ReadFile("/proc/sys/" + key); err == nil {
			info.SysctlParams[key] = strings.TrimSpace(string(content))
		}
	}

	// Network Interfaces
	links, err := netlink.LinkList()
	if err == nil {
		for _, link := range links {
			attrs := link.Attrs()
			// Skip loopback and dummy
			if attrs.Flags&net.FlagLoopback != 0 {
				continue
			}

			iface := InterfaceInfo{
				Name:    attrs.Name,
				MAC:     attrs.HardwareAddr.String(),
				MTU:     attrs.MTU,
				Offload: make(map[string]bool),
			}

			// Get IP
			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err == nil && len(addrs) > 0 {
				iface.IP = addrs[0].IP.String()
			}

			// Driver Info (Try via sysfs)
			// /sys/class/net/<iface>/device/driver/module -> points to module name
			// /sys/class/net/<iface>/device/uevent -> DRIVER=xxx
			if driver, err := getDriverName(attrs.Name); err == nil {
				iface.Driver = driver
			}

			// Try to get version if possible (often not in sysfs easily without ethtool ioctl)
			// We will leave version empty if not found, or implement ethtool ioctl later if critical.
			// For now, we stick to sysfs for safety.

			// Offload (Check /sys/class/net/<iface>/features/...)
			// This is complex to map exactly to TSO/GSO without ethtool, but we can try.
			// Actually, ethtool is the standard way. Since we can't use external binaries,
			// and implementing full ethtool netlink/ioctl is complex, we will try to read what we can.
			// For now, we'll mark them as unknown or try to read /sys/class/net/<iface>/features/* if they exist (kernel dependent).

			info.Interfaces = append(info.Interfaces, iface)
		}
	}

	return info, nil
}

func getDriverName(iface string) (string, error) {
	path := fmt.Sprintf("/sys/class/net/%s/device/uevent", iface)
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "DRIVER=") {
			return strings.TrimPrefix(line, "DRIVER="), nil
		}
	}
	return "", fmt.Errorf("driver not found")
}
