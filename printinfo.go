package main

import (
	"github.com/control-center/go-fsutils/diskinfo"
	"github.com/control-center/go-fsutils/btrfs"
	"fmt"
	"os"
)

var KB = uint64(1024)

func main() {
	fmt.Println(len(os.Args), os.Args)

	usage, err := diskinfo.NewDiskInfo(os.Args[1])
	if err != nil{
		fmt.Println(err);
		os.Exit(1);
	}
	fmt.Printf("type %v, %v\n", usage.FSType(), usage.Type)
	fmt.Printf("di %v\n", usage)
	fmt.Println("Free:", usage.Free()/(KB*KB))
//	fmt.Println("Available:", usage.Available()/(KB*KB))
	fmt.Println("Size:", usage.Size()/(KB*KB))
	fmt.Println("Used:", usage.Used()/(KB*KB))
	fmt.Println("Usage:", usage.Usage())

	fs, err := btrfs.GetFileSystem(os.Args[1])

	if err !=nil{
	   fmt.Println(err)
	   return
	}
	fmt.Printf("Btrfs: %+v\n", *fs)

}
