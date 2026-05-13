package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

var (
	db *sql.DB

	clients = make(map[*websocket.Conn]bool)
	mutex   sync.Mutex
)

func main() {

	initDB()

	app := fiber.New()

	app.Static("/", "./static")
	app.Get("/admin", func(c *fiber.Ctx) error {
		return c.SendFile("./static/admin.html")
	})

	app.Get("/menu", getMenu)
	app.Post("/sms", receiveSMS)
	app.Post("/clear", clearMenu)

	app.Get("/ws", websocket.New(wsHandler))

	app.Listen(getPort())
}

// ---------------- PORT ----------------
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

// ---------------- DB INIT ----------------
func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./menu.db")
	if err != nil {
		panic(err)
	}

	sqlStmt := `
	CREATE TABLE IF NOT EXISTS menus (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT,
		store TEXT,
		menu TEXT
	);
	`

	db.Exec(sqlStmt)
}

// ---------------- WS ----------------
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

func broadcast() {

	mutex.Lock()
	defer mutex.Unlock()

	for c := range clients {
		c.WriteMessage(websocket.TextMessage, []byte("update"))
	}
}

// ---------------- SMS ----------------
func receiveSMS(c *fiber.Ctx) error {

	type Req struct {
		Message string `json:"message"`
	}

	req := new(Req)
	c.BodyParser(req)

	parseMessage(req.Message)

	broadcast()

	return c.SendString("OK")
}

func parseMessage(msg string) {

	lines := strings.Split(msg, "\n")
	if len(lines) < 2 {
		return
	}

	date := time.Now().Format("2006-01-02")
	store := strings.TrimSpace(lines[0])

	// 기존 삭제
	db.Exec("DELETE FROM menus WHERE date=? AND store=?", date, store)

	for _, line := range lines[1:] {

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		db.Exec(
			"INSERT INTO menus(date, store, menu) VALUES(?, ?, ?)",
			date, store, line,
		)
	}

	fmt.Println("저장 완료:", store)
}

// ---------------- GET MENU ----------------
func getMenu(c *fiber.Ctx) error {

	date := time.Now().Format("2006-01-02")

	rows, _ := db.Query(
		"SELECT store, menu FROM menus WHERE date=?",
		date,
	)

	result := map[string][]string{}

	for rows.Next() {
		var store, menu string
		rows.Scan(&store, &menu)

		result[store] = append(result[store], menu)
	}

	return c.JSON(result)
}

// ---------------- CLEAR ----------------
func clearMenu(c *fiber.Ctx) error {

	date := time.Now().Format("2006-01-02")

	db.Exec("DELETE FROM menus WHERE date=?", date)

	broadcast()

	return c.SendString("OK")
}
