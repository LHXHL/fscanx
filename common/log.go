package common

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var Num int64
var End int64

// Results 带缓冲的结果队列：吸收终端打印/磁盘写的短时毛刺，避免背压直接传导到扫描 worker
var Results = make(chan *string, 256)
var LogSucTime int64
var LogErrTime int64
var WaitTime int64
var Silent bool
var Nocolor bool
var JsonOutput bool
var LogWG sync.WaitGroup

type JsonText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func init() {
	log.SetOutput(io.Discard)
	LogSucTime = time.Now().Unix()
	go SaveLog()
}

func LogSuccess(formatStr string, a ...any) {
	LogWG.Add(1)
	LogSucTime = time.Now().Unix()
	if a != nil {
		printStr := fmt.Sprintf(formatStr, a...)
		Results <- &printStr
	} else {
		Results <- &formatStr
	}
}

func SaveLog() {
	var fl *os.File
	var ferr error

	var colorFlag bool = true
	whiteStyle := color.New(color.FgWhite)
	yeStyleHi := color.New(color.FgHiYellow, color.Bold)
	cyanStyle := color.New(color.FgCyan)
	preTextStyle := color.New(color.FgWhite)
	redStyle := color.New(color.FgRed)

	for result := range Results {
		if fl == nil && IsSave {
			fl, ferr = os.OpenFile(Outputfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
			if ferr != nil {
				Printf("Create %s error, %v\n", Outputfile, ferr)
			}
		}
		if !Silent {
			withProgress(func() {
				if Nocolor {
					fmt.Println(*result)
				} else {
					if strings.HasPrefix(*result, "[+] InfoScan") {
						color.Green(*result)
					} else if strings.HasPrefix(*result, "[+] Product") || strings.HasPrefix(*result, "[*] JUMP") {
						// fmt.Println(*result)
						// color.Cyan(*result)
						if colorFlag == true {
							//color.HiYellow(*result)
							if strings.HasPrefix(*result, "[+] Product") {
								slice := strings.Split(*result, "\t")
								preTextStyle.Print(slice[0] + "\t" + slice[1] + "\t")
								yeStyleHi.Print(slice[2] + "\t")
								printFpsTail(strings.Join(slice[3:], "\t"), whiteStyle, redStyle)
							} else {
								color.White(*result)
							}

							colorFlag = false
						} else {
							if strings.HasPrefix(*result, "[+] Product") {
								slice := strings.Split(*result, "\t")
								preTextStyle.Print(slice[0] + "\t" + slice[1] + "\t")
								yeStyleHi.Print(slice[2] + "\t")
								printFpsTail(strings.Join(slice[3:], "\t"), cyanStyle, redStyle)
							} else {
								color.Cyan(*result)
							}

							colorFlag = true
						}

					} else if strings.Index(*result, "rdpscan") != -1 {
						color.Yellow(*result)
					} else if strings.HasPrefix(*result, "[+]") {
						color.Red(*result)
					} else {
						fmt.Println(*result)
					}
				}
			})
		}
		if IsSave {
			WriteFile(*result, fl)
		}
		LogWG.Done()
	}
	if fl != nil {
		fl.Close()
	}
}

// printFpsTail 打印 Product 行指纹段（第 4 个 \t 段起）并换行（BL-FOC-006）：
// [* 前缀 token（重点关注产品）用 redStyle，其余 token 与分隔符保持 baseStyle。
// 不含 "[*" 时整段一次 baseStyle.Println，输出与旧渲染逐字节一致（Contains 门控，
// 无 focus 行仅多一次子串扫描，且全部开销在 SaveLog 打印 goroutine，不触扫描 worker）。
func printFpsTail(fps string, baseStyle, redStyle *color.Color) {
	if !strings.Contains(fps, "[*") {
		baseStyle.Println(fps)
		return
	}
	for i, tok := range strings.Split(fps, ", ") {
		if i > 0 {
			baseStyle.Print(", ")
		}
		if strings.HasPrefix(tok, "[*") {
			redStyle.Print(tok)
		} else {
			baseStyle.Print(tok)
		}
	}
	fmt.Fprintln(color.Output)
}

func WriteFile(result string, fl *os.File) {
	defer func() {
		if r := recover(); r != nil {
			Printf("[CRITICAL] WriteFile panic: %v\n", r)
		}
	}()
	if JsonOutput {
		var scantype string
		var text string
		if strings.HasPrefix(result, "[+]") || strings.HasPrefix(result, "[*]") || strings.HasPrefix(result, "[-]") {
			//找到第二个空格的位置
			index := strings.Index(result[4:], " ")
			if index == -1 {
				scantype = "msg"
				text = result[4:]
			} else {
				scantype = result[4 : 4+index]
				text = result[4+index+1:]
			}
		} else {
			scantype = "msg"
			text = result
		}
		jsonText := JsonText{
			Type: scantype,
			Text: text,
		}
		jsonData, err := json.Marshal(jsonText)
		if err != nil {
			Println(err)
			jsonText = JsonText{
				Type: "msg",
				Text: result,
			}
			jsonData, err = json.Marshal(jsonText)
			if err != nil {
				Println(err)
				jsonData = []byte(result)
			}
		}
		jsonData = append(jsonData, []byte(",\n")...)
		_, _ = fl.Write(jsonData)
	} else {
		_, _ = fl.WriteString(result + "\n")
	}
	// 不做每行 fsync（BL-MEM-003）：WriteString 直写进入内核页缓存，write(2) 完成后
	// 进程终止（含 Ctrl+C）不丢已写行，仅断电可能丢尾部；逐行 fsync 的 ms 级成本
	// 会通过 Results 通道背压到所有扫描 worker。不引入 bufio：用户态缓冲进程死即丢。
}

func LogError(errinfo interface{}) {
	if WaitTime == 0 {
		//fmt.Printf("已完成 %v/%v %v \n", End, Num, errinfo)
	} else if (time.Now().Unix()-LogSucTime) > WaitTime && (time.Now().Unix()-LogErrTime) > WaitTime {
		//fmt.Printf("已完成 %v/%v %v \n", End, Num, errinfo)
		LogErrTime = time.Now().Unix()
	}
}

func CheckErrs(err error) bool {
	if err == nil {
		return false
	}
	errs := []string{
		"closed by the remote host", "too many connections",
		"i/o timeout", "EOF", "A connection attempt failed",
		"established connection failed", "connection attempt failed",
		"Unable to read", "is not allowed to connect to this",
		"no pg_hba.conf entry",
		"No connection could be made",
		"invalid packet size",
		"bad connection",
	}
	for _, key := range errs {
		if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(key)) {
			return true
		}
	}
	return false
}
