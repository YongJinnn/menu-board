package main

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	_ "modernc.org/sqlite"
)

var (
	menuMap = map[string][]string{}
	mutex   sync.Mutex

	clients = make(map[*websocket.Conn]bool)

	db *sql.DB
)

func main() {

	initDB()
	loadMenus()

	app := fiber.New()

	app.Static("/", "./static")

	app.Get("/menu", getMenu)

	app.Post("/sms", receiveSMS)

	app.Post("/clear", clearMenu)

	app.Get("/ws", websocket.New(wsHandler))

	app.Listen(":8080")
}

func initDB() {

	var err error

	db, err = sql.Open("sqlite", "./menu.db")

	if err != nil {
		panic(err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS menus (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		store TEXT,
		menu TEXT
	)
	`

	_, err = db.Exec(query)

	if err != nil {
		panic(err)
	}
}

func loadMenus() {

	mutex.Lock()
	defer mutex.Unlock()

	rows, err := db.Query("SELECT store, menu FROM menus")

	if err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {

		var store string
		var menu string

		rows.Scan(&store, &menu)

		menuMap[store] = append(menuMap[store], menu)
	}
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

	db.Exec("DELETE FROM menus WHERE store = ?", store)

	menuMap[store] = nil

	for _, line := range lines[1:] {

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		menuMap[store] = append(menuMap[store], line)

		db.Exec(
			"INSERT INTO menus(store, menu) VALUES(?, ?)",
			store,
			line,
		)
	}

	fmt.Println("저장 완료:", store)
}

func getMenu(c *fiber.Ctx) error {

	mutex.Lock()
	defer mutex.Unlock()

	return c.JSON(menuMap)
}

func clearMenu(c *fiber.Ctx) error {

	mutex.Lock()
	defer mutex.Unlock()

	menuMap = map[string][]string{}

	_, err := db.Exec("DELETE FROM menus")

	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	for client := range clients {
		client.WriteJSON(menuMap)
	}

	fmt.Println("전체 메뉴 삭제 완료")

	return c.SendString("OK")
}
