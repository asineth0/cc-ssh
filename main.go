package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

type Message struct {
	Type string          `json:"t"`
	Data json.RawMessage `json:"d"`
}

func main() {
	var upgrader websocket.Upgrader

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(rw, r, nil)

		if err != nil {
			return
		}

		cmd := exec.Command("ssh", "-t", "ryuu.wg.asineth.me", "tmux", "a")

		ptmx, _ := pty.StartWithSize(cmd, &pty.Winsize{
			Cols: 164,
			Rows: 81,
		})

		buf := make([]byte, 1000)
		for {
			n, err := ptmx.Read(buf)

			if err != nil {
				c.Close()
				break
			}

			mode := "raw"
			ansiBuf := ""

			for _, char := range buf[:n] {
				if mode == "raw" {
					if char == 0x1b {
						mode = "ansi"
						ansiBuf = ""
						continue
					}

					if char == 0x0a {
						c.WriteMessage(websocket.BinaryMessage, []byte("print(\"\")"))
						time.Sleep(1 * time.Millisecond)
						continue
					}

					c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("term.write(\"\\x%02x\")", char)))
					time.Sleep(1 * time.Millisecond)
				}

				if mode == "ansi" {
					if char == '[' {
						continue
					}

					if char == 0x1b {
						ansiBuf = ""
						continue
					}

					ops := []int{}
					for _, opText := range strings.Split(ansiBuf, ";") {
						op, _ := strconv.Atoi(opText)
						ops = append(ops, op)
					}

					if char == 'm' {
						for _, op := range ops {
							if op == 0 {
								c.WriteMessage(websocket.BinaryMessage, []byte("term.setBackgroundColor(colors.black)"))
								time.Sleep(1 * time.Millisecond)
								c.WriteMessage(websocket.BinaryMessage, []byte("term.setTextColor(colors.white)"))
								time.Sleep(1 * time.Millisecond)
								continue
							}

							colors := []string{
								"black",
								"red",
								"green",
								"yellow",
								"blue",
								"magenta",
								"cyan",
								"white",
							}

							color := ""
							colorType := "Text"

							if op > 29 && op < 38 {
								color = colors[op-30]
							}

							if op > 39 && op < 48 {
								color = colors[op-40]
								colorType = "Background"
							}

							if color != "" {
								c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("term.set%sColor(colors.%s)", colorType, color)))
								time.Sleep(1 * time.Millisecond)
							}
						}

						mode = "raw"
					}

					//Move cursor up the indicated # of rows.
					if char == 'A' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()term.setCursorPos(x,math.max(1,y-%d))", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor down the indicated # of rows.
					if char == 'B' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()xS,yS=term.getSize()term.setCursorPos(x,math.min(yS,y+%d))", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor right the indicated # of columns.
					if char == 'C' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()xS=term.getSize()term.setCursorPos(math.min(xS,x+%d),y)", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor left the indicated # of columns.
					if char == 'D' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()term.setCursorPos(math.max(1,x-%d),y)", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor down the indicated # of rows, to column 1.
					if char == 'E' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()term.setCursorPos(1,math.max(1,y+%d))", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor up the indicated # of rows, to column 1.
					if char == 'F' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()term.setCursorPos(1,math.max(1,y-%d))", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor to indicated column in current row.
					if char == 'G' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()term.setCursorPos(%d,y)", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Move cursor to the indicated row, column (origin at 1,1).
					if char == 'H' {
						x := 1
						y := 1
						if len(ops) > 1 {
							x = int(math.Min(math.Max(float64(1), float64(ops[1])), 164))
							y = int(math.Min(math.Max(float64(1), float64(ops[0])), 81))
						}
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("term.setCursorPos(%d,%d)", x, y)))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					//Erase line (default: from cursor to end of line).
					//ESC [ 1 K: erase from start of line to cursor.
					//ESC [ 2 K: erase whole line.
					if char == 'K' {
						if len(ops) < 1 || ops[0] == 1 {
							c.WriteMessage(websocket.BinaryMessage, []byte("x,y=term.getCursorPos()term.setCursorPos(1,y)for i=1,x do term.write(\" \")"))
						}

						if len(ops) > 0 && ops[0] == 2 {
							c.WriteMessage(websocket.BinaryMessage, []byte("x,y=term.getCursorPos()term.clearLine()term.setCursorPos(x,y)"))
						}

						mode = "raw"
					}

					//Erase the indicated # of characters on current line.
					if char == 'X' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()for i=1,%d do term.write(\" \") end term.setCursorPos(x,y)", ops[0])))
						mode = "raw"
					}

					//Move cursor to the indicated row, current column.
					if char == 'd' {
						c.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("x,y=term.getCursorPos()term.setCursorPos(x,%d)", ops[0])))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					if char >= 'A' && mode != "raw" {
						safeChar, _ := json.Marshal(string(char))
						safeAnsiBuf, _ := json.Marshal(ansiBuf)
						log.Printf("!unhandled(safeChar:%s safeAnsiBuf:%s)\n", safeChar, safeAnsiBuf)
					}

					ansiBuf += string(char)

					if ansiBuf == "?25l" {
						c.WriteMessage(websocket.BinaryMessage, []byte("term.setCursorBlink(false)"))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}

					if ansiBuf == "?25h" {
						c.WriteMessage(websocket.BinaryMessage, []byte("term.setCursorBlink(true)"))
						time.Sleep(1 * time.Millisecond)
						mode = "raw"
					}
				}
			}
		}
	})

	http.ListenAndServe(":8000", nil)
}
