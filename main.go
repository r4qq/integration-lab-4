package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// globalna struktura "zależności" - wskaźnik do definicji bazy danych. Dzięki niej zamiast
// używać globalnej zmiennej DB, każda funkcja będzie metodą tej struktury
type Env struct {
	db *gorm.DB
}

// struktura będąca jednocześnie modelem posta dla db
type Post struct {
	gorm.Model        // dodaje automatycznie pola id, createdAt, updatedAt, deletedAt
	Title      string `gorm:"type:text;not null" form:"title" binding:"required"`
	Body       string `gorm:"type:text;not null" form:"body"  binding:"required"`
	UserID     uint
	Author     User `gorm:"foreignKey:UserID"`
}

// struktura będąca jednocześnie modelem użytkownika dla db
type User struct {
	gorm.Model        // dodaje automatycznie pola id, createdAt, updatedAt, deletedAt
	Username   string `gorm:"unique;not null" form:"user" binding:"required"`
	Posts      []Post
}

// struktura przechowująca konfigurację bazy danych
type DBconfig struct {
	Host     string
	User     string
	Password string
	Name     string
	Port     string
}

func LoadConfig() string {
	db := &DBconfig{
		Host:     os.Getenv("DB_HOST"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		Port:     os.Getenv("DB_PORT"),
	}

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		db.Host, db.User, db.Password, db.Name, db.Port)
}

// obsługa i generowanie strony głównej (wszystkie posty)
func (e *Env) getAllPosts(c *gin.Context) {
	var allPosts []Post

	if err := e.db.Preload("Author").Order("created_at desc").Find(&allPosts).Error; err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.HTML(http.StatusOK, "index.templ", gin.H{
		"title": "golang blog",
		"posts": allPosts,
	})
}

// obsługa i generowanie strony dla danego posta
func (e *Env) getPostById(c *gin.Context) {
	var post Post
	if err := e.db.Preload("Author").First(&post, c.Param("id")).Error; err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.HTML(http.StatusOK, "post.templ", gin.H{
		"title": "golang blog post",
		"post":  post,
	})
}

// obsługa usuwania posta
func (e *Env) deletePost(c *gin.Context) {
	id := c.Param("id")
	if err := e.db.Delete(&Post{}, id).Error; err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/")
}

// obsługa dodawania posta
func (e *Env) createPost(c *gin.Context) {
	var post Post
	if err := c.ShouldBind(&post); err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusAlreadyReported, err.Error())
		return
	}

	if err := e.db.Create(&post).Error; err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.Redirect(http.StatusFound, "/")
}

// obsługa dodawanie usera
func (e *Env) createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBind(&user); err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := e.db.Create(&user).Error; err != nil {
		log.Printf("%s", err.Error())
		renderError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/")
}

// helper do generowania podstrony dla błędu
func renderError(c *gin.Context, status int, message string) {
	c.HTML(status, "error.templ", gin.H{
		"code":    status,
		"message": message,
	})
}

// helper do generowania połączenia z bazą
func setupDatabase() *gorm.DB {
	var err error
	var db *gorm.DB

	dsn := LoadConfig()

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("connection to db failed: %v", err)
	}

	err = db.AutoMigrate(&User{}, &Post{})
	if err != nil {
		log.Fatalf("migartion to db failed: %v", err)
	}

	// przykladowi userzy
	var users = []User{
		{Username: "glenda"},
		{Username: "pike"},
	}

	var count int64
	db.Model(&User{}).Count(&count)
	if count == 0 {
		db.Create(&users)
		log.Println("users seeded")
	}

	// przykładowe posty
	var posts = []Post{
		{Title: "9front", Body: "the front fell off", UserID: users[0].ID},
		{Title: "go", Body: "lang", UserID: users[1].ID},
	}

	db.Model(&Post{}).Count(&count)
	if count == 0 {
		db.Create(&posts)
		log.Println("users seeded")
	}
	return db
}

func main() {
	gin.SetMode(gin.DebugMode)

	conn := setupDatabase()
	env := &Env{db: conn}

	router := gin.Default()
	router.LoadHTMLGlob("templates/*.templ")

	//globalny css
	router.Static("/css", "./css")

	//strona główna
	router.GET("/", env.getAllPosts)

	//podstrony dla posczególnych postów
	router.GET("/posts/:id", env.getPostById)

	//obługa tworzenia nowego posta
	router.POST("/posts/new", env.createPost)

	//usuwanie posta
	router.POST("/posts/:id/delete", env.deletePost)

	router.POST("/users/new", env.createUser)

	//formularz dla nowego usera
	router.GET("/users/new", func(c *gin.Context) {
		c.HTML(http.StatusOK, "addu.templ", gin.H{
			"title": "Add New User",
		})
	})

	//formularz dla nowego posta
	router.GET("/posts/new", func(c *gin.Context) {
		var usersList []User
		if err := env.db.Find(&usersList).Error; err != nil {
			log.Printf("%s", err.Error())
			renderError(c, http.StatusInternalServerError, err.Error())
			return
		}

		c.HTML(http.StatusOK, "addp.templ", gin.H{
			"title":     "Add New Post",
			"usersList": usersList,
		})
	})

	//domyślna strona w przypadku nie znalezienia danego linku
	router.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "error.templ", gin.H{
			"code":    http.StatusNotFound,
			"message": "Page not found",
		})
	})

	router.Run("0.0.0.0:8000")
}
