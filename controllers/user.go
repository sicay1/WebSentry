package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/websentry/websentry/models"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"net/http"
	"time"
)

const (
	verificationCodeLength = 6
)

// UserGetSignUpVerification gets user email and password, generate Verification code and wait to be validated
func UserGetSignUpVerification(c *gin.Context) {
	gUsername := c.Query("username")
	db := c.MustGet("mongo").(*mgo.Database)

	// check existence of the user
	userAlreadyExist, err := models.CheckUserExistence(db, 0, gUsername)
	if err != nil {
		panic(err)
	}
	if userAlreadyExist {
		c.JSON(http.StatusOK, gin.H{
			"code": -2,
			"msg": "Wrongn parameter: User already exists",
		})
		return
	}

	if err = models.EnsureUserVerificationsIndex(db); err != nil {
		panic(err)
	}

	var verificationCode string

	userVerificationExist, err := models.CheckUserExistence(db, 1, gUsername)

	// TODO: test
	if userVerificationExist {
		// fetched verification code before
		result := models.UserVerification{}
		err = models.GetUser(db, 1, gUsername, &result)
		if err != nil {
		panic(err)
		}

		verificationCode = result.VerificationCode
		err = models.GetUserCollection(db, 1).Update(
			bson.M{"username": gUsername},
			bson.M{"$set": bson.M{"createdAt": time.Now()}},
		)
	} else {
		verificationCode = generateVerificationCode()
		err = models.GetUserCollection(db, 1).Insert(&models.UserVerification{
			Username:       gUsername,
			VerificationCode: verificationCode,
			CreatedAt:      time.Now(),
		})
	}
	if err != nil {
		panic(err)
	}

	SendVerificationEmail(gUsername, verificationCode)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg": "OK",
	})
}

// UserCreateWithVerification checks verification code and create the user in the user database
func UserCreateWithVerification(c *gin.Context) {
	// gUsername := c.Query("username")
	// gPassword := c.Query("password")
	// gVerificationCode := c.Query("verificationcode")

	// check if the user exist in UserVerifications table

	// check if the verification code is correct

	// check if it is already in the Users table


}


// generateVerificationCode outputs a random 6-digit code
func generateVerificationCode() string {
	numBytes := [...]byte{'1', '2', '3', '4', '5', '6', '7', '8', '9', '0'}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	rst := make([]byte, verificationCodeLength)

	for i := range rst {
		rst[i] = numBytes[r.Intn(len(numBytes))]
	}

	return string(rst)
}
