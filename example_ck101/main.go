package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	. "github.com/kkdai/photomgr"
)

func printPageResult(p *CK101, count int) {
	for i := 0; i < count; i++ {
		title := p.GetPostTitleByIndex(i)
		likeCount := p.GetPostStarByIndex(i)
		fmt.Printf("%d:[%d★]%s\n", i, likeCount, title)
	}
	fmt.Printf("(o: open file in fider, s: top page, n:next, p:prev, quit: quit program)\n")
}

type NullWriter int

func (NullWriter) Write([]byte) (int, error) { return 0, nil }

func main() {

	log.SetOutput(new(NullWriter))
	c := NewCK101()

	usr, _ := user.Current()
	c.BaseDir = fmt.Sprintf("%v/Pictures/iloveCK101", usr.HomeDir)

	var workerNum int
	rootCmd := &cobra.Command{
		Use:   "iloveCK101",
		Short: "Download all the images in given post url",
		Run: func(cmd *cobra.Command, args []string) {
			page := 0
			pagePostCoubt := 0
			pagePostCoubt = c.ParseCK101PageByIndex(page)
			printPageResult(c, pagePostCoubt)

			scanner := bufio.NewScanner(os.Stdin)
			quit := false

			for !quit {
				fmt.Print("CK101:> ")

				if !scanner.Scan() {
					break
				}

				line := scanner.Text()
				parts := strings.Split(line, " ")
				cmd := parts[0]
				args := parts[1:]

				switch cmd {
				case "quit":
					quit = true
				case "n":
					page = page + 1
					pagePostCoubt = c.ParseCK101PageByIndex(page)
					printPageResult(c, pagePostCoubt)
				case "p":
					if page > 0 {
						page = page - 1
					}
					pagePostCoubt = c.ParseCK101PageByIndex(page)
					printPageResult(c, pagePostCoubt)
				case "s":
					page = 0
					pagePostCoubt = c.ParseCK101PageByIndex(page)
					printPageResult(c, pagePostCoubt)
				case "o":
					open.Run(filepath.FromSlash(c.BaseDir))
				case "d":
					if len(args) == 0 {
						fmt.Println("You don't input any article index. Input as 'd 1'")
						continue
					}

					index, err := strconv.Atoi(args[0])
					if err != nil {
						fmt.Println(err)
						continue
					}

					url := c.GetPostUrlByIndex(index)

					if int(index) >= len(url) {
						fmt.Println("Invalid index")
						continue
					}

					if c.HasValidURL(url) {
						c.Crawler(url, 25)
						fmt.Println("Done!")
					} else {
						fmt.Println("Unsupport url:", url)
					}
				default:
					fmt.Println("Unrecognized command:", cmd, args)
				}
			}
		},
	}

	rootCmd.Flags().IntVarP(&workerNum, "worker", "w", 25, "Number of workers")
	rootCmd.Execute()
}
