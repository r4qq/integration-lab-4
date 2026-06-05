package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// uruchomienie testowego kontenera żeby testować na "produkcyjnej" bazie danych
func testDB(t *testing.T) (*gorm.DB, func()) {
	ctx := context.Background()

	postgresContainer, err := tcPostgres.Run(ctx,
		"postgres:15-alpine",
		tcPostgres.WithDatabase("test_db"),
		tcPostgres.WithUsername("test_user"),
		tcPostgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(time.Second*10),
		),
	)
	if err != nil {
		t.Fatalf("failed to start posgres container: %v", err)
	}

	testDsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get dsn string: %v", err)
	}

	db, err := gorm.Open(gormPostgres.Open(testDsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	err = db.AutoMigrate(&User{}, &Post{})
	if err != nil {
		t.Fatalf("failed to automigrate: %v", err)
	}

	cleanup := func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate the test databse: %v", err)
		}
	}

	return db, cleanup
}

func TestMain(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	env := &Env{db: db}

	gin.SetMode(gin.TestMode)

	//testowy router
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

	t.Run("GET / - pusta lista postów (index.templ)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)

		//sprawdzenie zwrócony kod http to 200
		assert.Equal(t, http.StatusOK, w.Code)
		//sprawdza czy w danych get jest string "golang blog" (tytuł strony)
		assert.Contains(t, w.Body.String(), "golang blog")
	})

	t.Run("POST /posts/user z prawidłowymi danymi", func(t *testing.T) {
		//dane do testowego "formularza"
		formData := url.Values{}
		formData.Set("user", "test_user")
		body := strings.NewReader(formData.Encode())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/users/new", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		router.ServeHTTP(w, req)

		//sprawdzenie zwrócony kod http to 302
		assert.Equal(t, http.StatusFound, w.Code)

		//sprawdza czy rekord jest w bazie
		var user User
		err := env.db.First(&user, "username = ?", "test_user").Error
		assert.NoError(t, err)
	})

	t.Run("POST /posts/new z pustym polem title", func(t *testing.T) {
		//dane do testowego "formularza"
		formData := url.Values{}
		formData.Set("user", "1")
		formData.Set("title", "")
		formData.Set("body", "test_body")
		body := strings.NewReader(formData.Encode())

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/posts/new", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		router.ServeHTTP(w, req)

		//sprawdzenie zwrócony kod http to 400
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
