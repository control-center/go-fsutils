package btrfs

import (
	"bufio"
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type FileSystem struct {
	Label        string
	UUID         string
	TotalDevices uint64
	UsedBytes    uint64
	Version      string
	devices      []Device
	subvolumes   []Subvolume
	dfData       []DFData
}

func readLines(reader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(reader)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return []string{}, err
	}
	return lines, nil
}

func GetFileSystem(path string) (*FileSystem, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	fs, err := readFileSystem(path)
	if err != nil {
		return nil, err
	}

	//btrfs fi df <path>, parse info into structs
	cmd := exec.Command("btrfs", "fi", "df", path)
	dfOut, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	fmt.Println(string(dfOut[:]))

	//also btrfs subvolume list
	cmd = exec.Command("btrfs", "subvolume", "list", path)
	svListOut, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	fmt.Println(string(svListOut[:]))

	return fs, nil
}

func readFileSystem(path string) (*FileSystem, error) {
	//do a btrfs fi show <path>
	cmd := exec.Command("btrfs", "fi", "show", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	showLines, err := readLines(stdout)
	if err != nil {
		return nil, err
	}
	errLines, err := readLines(stderr)
	if err != nil {
		return nil, err
	}

	if len(errLines) != 0 {
		return nil, fmt.Errorf("Error reading btrfs fi show: %v", errLines)
	}

	return parseFSShow(showLines)
}

func parseFSShow(lines []string) (*FileSystem, error) {
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected output, check permissions: %v", strings.Join(lines, "\n"))
	}
	fmt.Println("fs show output:")
	//Should be of format:
	//	Label: none  uuid: b7c23711-6b9e-46a8-b451-4b3f79c7bc46
	//		Total devices 2 FS bytes used 14.67GiB
	//		devid    1 size 40.00GiB used 16.01GiB path /dev/sdc1
	//		devid    2 size 40.00GiB used 16.01GiB path /dev/sdd1
	//
	//	Btrfs v3.12
	var version, label, uuid string
	var totalDevices, usedBytes uint64
	var err error
	devices := []Device{}

	lineCount := len(lines)
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		switch lineNum {
		case 0:
			if fields[0] != "Label:" {
				return nil, fmt.Errorf("expected label and uuid, got: %v", line)
			} else {
				if len(fields) != 4 {
					return nil, fmt.Errorf("unexpected fields for FS info: %v", line)
				}
				label = fields[1]
				uuid = fields[3]
			}
		case lineCount - 1:
			//get last line for version
			if fields[0] != "Btrfs" {
				return nil, fmt.Errorf("expected btrfs version, got: %v", line)
			} else {
				if len(fields) != 2 {
					return nil, fmt.Errorf("unexpected fields for version output: %v", line)
				}
				version = fields[1]
			}
		case 1:
			if fields[0] != "Total" && fields[1] != "devices" {
				return nil, fmt.Errorf("Expected Total Device content, got: %v", line)
			} else {
				if len(fields) != 7 {
					return nil, fmt.Errorf("unexpected fields for total device line: %v", line)
				}
				if totalDevices, err = strconv.ParseUint(fields[2], 10, 64); err != nil {
					return nil, err
				}
				size := fields[6]
				if usedBytes, err = parseSize(size); err != nil {
					return nil, err
				}
			}
		default:
			if len(fields) == 0 {continue}
			if fields[0] != "devid" {
				return nil, fmt.Errorf("expected btrfs device content, got: %v", line)
			}
			if len(fields) != 8 {
				return nil, fmt.Errorf("unexpected fields for device line: %v", line)
			}

			var size, used uint64
			if size, err = parseSize(fields[3]); err != nil {
				return nil, fmt.Errorf("error parsing device size: %v", err)
			}
			if used, err = parseSize(fields[5]); err != nil {
				return nil, fmt.Errorf("error parsing device used bytes: %v", err)
			}
			device := Device{DevID: fields[1], Path: fields[7], Size: size, Used: used}
			devices = append(devices, device)
		}
	}

	for _, line := range lines {
		fmt.Println(line)
	}
	fs := FileSystem{Label: label,
		UUID:         uuid,
		TotalDevices: totalDevices,
		UsedBytes:    usedBytes,
		Version:      version,
		subvolumes:   []Subvolume{},
		dfData:       []DFData{},
		devices:      devices}

	return &fs, nil
}

func parseSize(size string) (uint64, error) {
	return humanize.ParseBytes(size)
}

func (fs *FileSystem) TotalBytes() uint64 {
	//add Device sizes
	var total uint64 = 0
	for _, val := range fs.devices {
		total += val.Size
	}
	return total
}

func (fs *FileSystem) AllocatedBytes() uint64 {
	//Add device used values
	var total uint64 = 0
	for _, val := range fs.devices {
		total += val.Used
	}
	return total
}

func (fs *FileSystem) GetUsedBytes() (uint64, error) {
	//Add DFData UsedTotal
	var total uint64 = 0
	for _, val := range fs.dfData {
		if used, err := val.UsedTotal(); err != nil {
			return 0, err
		} else {
			total += used
		}
	}
	return total, nil
}

func (fs *FileSystem) DF() []DFData {
	return fs.dfData
}

func (fs *FileSystem) Devices() []Device {
	return fs.devices
}

func (fs *FileSystem) Subvolumes() []Subvolume {
	return fs.subvolumes
}

type Device struct {
	DevID string
	Size  uint64
	Used  uint64
	Path  string
}

type DFData struct {
	DataType string
	Level    string
	Total    uint64
	Used     uint64
}

func (df *DFData) UsedTotal() (uint64, error) {
	//DFData "used" values, account for "raid" levels
	switch df.Level {
	case "single":
		return df.Used, nil
	case "dup", "raid-1":
		return df.Used * 2, nil

	}
	return 0, fmt.Errorf("Unknown level %v", df.Level)
}

type Subvolume struct {
	Name          string
	UUID          string
	ParentUUID    string
	CreationTime  time.Time
	ID            string
	Gen           uint32
	GenAtCreation uint32
	Parent        uint32
	TopLevel      uint32
	Path          string
	//Flags  TODO:
	//Snapshots TODO:
}
