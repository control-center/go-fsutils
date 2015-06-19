package btrfs

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type FileSystem struct {
	Label        string
	UUID         string
	TotalDevices uint
	UsedBytes    uint64
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
	fmt.Println("fs show output:")
	for _, line := range lines {
		fmt.Println(line)
	}
	fs := FileSystem{subvolumes: []Subvolume{}, dfData: []DFData{}, devices: []Device{}}

	return &fs, nil
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
	DevID uint
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
