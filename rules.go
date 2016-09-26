package main

import (
	"net/http"

	"github.com/fasthall/cappa/redis"

	"github.com/gin-gonic/gin"
	//"github.com/nu7hatch/gouuid"
)

func rulesPOST(c *gin.Context) {
	event := c.PostForm("event")
	action := c.PostForm("action")
	task := c.PostForm("task")
	image := redis.Get("tasks", task)
	if image == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"task":   task,
			"image":  "not found",
			"result": "",
		})
		return
	}

	//rule, err := uuid.NewV4()
	//if err != nil {
	//	c.JSON(http.StatusBadRequest, gin.H{
	//		"message": "couldn't generate new event ID",
	//	})
	//	return
	//}
	redis.Set("rules", event+"_"+action, image)
}