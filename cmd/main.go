package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maksymshtarkberg/music-player-go/pkg/models"
	"github.com/nats-io/nats.go"
)

var nc *nats.Conn

func main() {
	var err error
	nc, err = nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	router := gin.Default()

	router.POST("/api/v1/user/reg", RegisterUserNats)
	router.POST("/api/v1/user/auth", AuthUserNats)

	router.Run(":3000")
}

func RegisterUserNats(c *gin.Context) {
	var jsonData map[string]interface{}
	if err := c.ShouldBindJSON(&jsonData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := nc.Request("auth.register", encode(jsonData), nats.DefaultTimeout*5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, decode(response.Data))
}

func AuthUserNats(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := nc.Request("auth.authenticate", encode(user), nats.DefaultTimeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, decode(response.Data))
}

func encode(data interface{}) []byte {
	encoded, _ := json.Marshal(data)
	return encoded
}

func decode(data []byte) map[string]interface{} {
	var decoded map[string]interface{}
	_ = json.Unmarshal(data, &decoded)
	return decoded
}
