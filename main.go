package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

var (
	menuMap = map[string][]string{}
	mutex   sync.Mutex

	clients = make(map[*websocket.Conn]bool)

	fileName = "menu.json"
)

func main() {

	loadMenu()

	app := fiber.New()

	app.Static("/", "./static")

	app.Get("/menu", getMenu)

	app.Post("/sms", receiveSMS)

	app.Post("/clear", clearMenu)

	app.Get("/ws", websocket.New(wsHandler))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./static/index.html")
	})

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	app.Listen(":" + port)
}

func loadMenu() {

	data, err := os.ReadFile(fileName)

	if err != nil {

		menuMap = map[string][]string{}

		return
	}

	json.Unmarshal(data, &menuMap)
}

func saveMenu() {

	data, _ := json.MarshalIndent(menuMap, "", "  ")

	os.WriteFile(fileName, data, 0644)
}

func wsHandler(c *websocket.Conn) {

	clients[c] = true

	defer func() {

		delete(clients, c)

		c.Close()
	}()

	for {

		if _, _, err := c.ReadMessage(); err != nil {

			break
		}
	}
}

func broadcastMenu() {

	mutex.Lock()
	defer mutex.Unlock()

	for client := range clients {

		client.WriteJSON(menuMap)
	}
}

func receiveSMS(c *fiber.Ctx) error {

	type Request struct {
		Message string `json:"message"`
	}

	req := new(Request)

	if err := c.BodyParser(req); err != nil {

		return c.Status(400).SendString(err.Error())
	}

	fmt.Println("문자 수신:")
	fmt.Println(req.Message)

	parseMessage(req.Message)

	broadcastMenu()

	return c.SendString("OK")
}

func parseMessage(msg string) {

	mutex.Lock()
	defer mutex.Unlock()

	lines := strings.Split(msg, "\n")

	if len(lines) < 2 {

		return
	}

	store := strings.TrimSpace(lines[0])

	menuMap[store] = nil

	for _, line := range lines[1:] {

		line = strings.TrimSpace(line)

		if line == "" {

			continue
		}

		menuMap[store] = append(menuMap[store], line)
	}

	saveMenu()

	fmt.Println("저장 완료:", store)
}

func getMenu(c *fiber.Ctx) error {

	return c.JSON(menuMap)
}

func clearMenu(c *fiber.Ctx) error {

	mutex.Lock()
	defer mutex.Unlock()

	menuMap = map[string][]string{}

	saveMenu()

	for client := range clients {

		client.WriteJSON(menuMap)
	}

	fmt.Println("전체 메뉴 삭제 완료")

	return c.SendString("OK")
}
