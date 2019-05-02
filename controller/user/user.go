package user

import (
	"fmt"
	"net/http"
	"time"

	"goblog/config"
	"goblog/controller/common"
	"goblog/model"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

func Signin(c *gin.Context) {
	SendErrJSON := common.SendErrJSON

	userName := c.PostForm("userName")
	password := c.PostForm("password")
	var user model.User
	if err := model.DB.Where("user_name = ?", userName).First(&user).Error; err != nil {
		SendErrJSON("账号不存在", c)
		return
	}

	if user.CheckPassword(password) {
		setLogin(c, user)
	} else {
		SendErrJSON("账号或密码错误", c)
	}
}

func Signup(c *gin.Context) {
	SendErrJSON := common.SendErrJSON

	userName := c.PostForm("userName")
	password := c.PostForm("password")
	var user model.User
	if err := model.DB.Where("user_name = ?", userName).Find(&user).Error; err == nil {
		SendErrJSON("用户名已被注册", c)
		return
	}

	var newUser model.User
	newUser.UserName = userName
	newUser.AddTime = time.Now().Unix()
	newUser.Password = newUser.EncryptPassword(password)

	//检查是否第一个注册，第一个注册的设置为管理员
	if err := model.DB.First(&user).Error; err != nil {
		newUser.IsAdmin = 1
	}

	if err := model.DB.Create(&newUser).Error; err != nil {
		SendErrJSON("创建用户失败", c)
		return
	}

	setLogin(c, newUser)
}

func Signout(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{
		"errNo": model.ErrorCode.SUCCESS,
		"msg":   "success",
		"data":  gin.H{},
	})
}

func setLogin(c *gin.Context, user model.User) {
	SendErrJSON := common.SendErrJSON

	lastLogin := time.Now().Unix()
	if err := model.DB.Model(&user).Update("last_login", lastLogin).Error; err != nil {
		SendErrJSON("登录内部错误", c)
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": user.ID,
	})
	tokenString, err := token.SignedString([]byte(config.ServerConfig.TokenSecret))
	if err != nil {
		fmt.Println(err.Error())
		SendErrJSON("内部错误", c)
		return
	}

	//清理密码
	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"token": tokenString,
			"user":  user,
		},
	})
}
