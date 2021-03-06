package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/websentry/websentry/models"
	"github.com/websentry/websentry/utils"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"time"
)

const (
	verificationCodeLength = 6
)

func UserInfo(c *gin.Context) {
	db := c.MustGet("mongo").(*mgo.Database)
	result := models.User{}
	err := models.GetUserById(db, c.MustGet("userId").(bson.ObjectId), &result)
	if err != nil {
		panic(err)
	}

	JsonResponse(c, CodeOK, "", gin.H{
		"email": result.Email,
	})
	return
}

func UserLogin(c *gin.Context) {
	gEmail := c.DefaultQuery("email", "")
	gPassword := c.DefaultQuery("password", "")
	db := c.MustGet("mongo").(*mgo.Database)

	if gEmail == "" {
		JsonResponse(c, CodeWrongParam, "Email required", nil)
		return
	}

	if gPassword == "" {
		JsonResponse(c, CodeWrongParam, "Password required", nil)
		return
	}

	// limits login attempts
	// TODO: add captcha
	if utils.CheckLoginAvailability(gEmail, c.ClientIP()) == false {
		JsonResponse(c, CodeExceededLimits, "", nil)
		// "expireDuration": utils.LoginExpireDuration,
		return
	}

	// check if the user exists
	userExist, err := models.CheckUserExistence(db, 0, gEmail)
	if err != nil {
		panic(err)
	}
	if !userExist {
		JsonResponse(c, CodeNotExist, "sign up required", nil)
		return
	}

	// check password
	result := models.User{}
	err = models.GetUserByEmail(db, 0, gEmail, &result)
	if err != nil {
		panic(err)
	}

	if !models.CheckPassword(gPassword, result.Password) {
		JsonResponse(c, CodeAuthError, "incorrect email/password", nil)
		return
	}

	JsonResponse(c, CodeOK, "", gin.H{
		"token": utils.TokenGenerate(result.Id.Hex()),
	})
}

// UserGetSignUpVerification gets user email and password, generate Verification code and wait to be validated
func UserGetSignUpVerification(c *gin.Context) {
	gEmail := c.DefaultQuery("email", "")
	db := c.MustGet("mongo").(*mgo.Database)

	// TODO: email check
	if gEmail == "" {
		JsonResponse(c, CodeWrongParam, "Email required", nil)
		return
	}

	// check existence of the user
	userAlreadyExist, err := models.CheckUserExistence(db, 0, gEmail)
	if err != nil {
		panic(err)
	}
	if userAlreadyExist {
		JsonResponse(c, CodeAlreadyExist, "", nil)
		return
	}

	if err = models.EnsureUserVerificationsIndex(db); err != nil {
		panic(err)
	}

	var verificationCode string

	userVerificationExist, err := models.CheckUserExistence(db, 1, gEmail)
	if err != nil {
		panic(err)
	}

	if userVerificationExist {
		// fetched verification code before
		result := models.UserVerification{}
		err = models.GetUserByEmail(db, 1, gEmail, &result)
		if err != nil {
			panic(err)
		}

		verificationCode = result.VerificationCode
		err = models.GetUserCollection(db, 1).Update(
			bson.M{"email": gEmail},
			bson.M{"$set": bson.M{"createdAt": time.Now()}},
		)
	} else {
		verificationCode = generateVerificationCode()
		err = models.GetUserCollection(db, 1).Insert(&models.UserVerification{
			Email:            gEmail,
			VerificationCode: verificationCode,
			CreatedAt:        time.Now(),
		})
	}
	if err != nil {
		panic(err)
	}

	utils.SendVerificationEmail(gEmail, verificationCode)

	JsonResponse(c, CodeOK, "", nil)
}

// UserCreateWithVerification checks verification code and create the user in the user database
func UserCreateWithVerification(c *gin.Context) {
	gEmail := c.DefaultQuery("email", "")
	gPassword := c.DefaultQuery("password", "")
	gVerificationCode := c.DefaultQuery("verification", "")

	if gEmail == "" {
		JsonResponse(c, CodeWrongParam, "Email required", nil)
		return
	}

	if gPassword == "" {
		JsonResponse(c, CodeWrongParam, "Password required", nil)
		return
	}

	if gVerificationCode == "" {
		JsonResponse(c, CodeWrongParam, "Verification code required", nil)
		return
	}

	db := c.MustGet("mongo").(*mgo.Database)

	// check if it is already in the Users table
	userExist, err := models.CheckUserExistence(db, 0, gEmail)
	if err != nil {
		panic(err)
	}

	if userExist {
		JsonResponse(c, CodeAlreadyExist, "", nil)
		return
	}

	// check if the user exist in UserVerifications table
	userVerificationExist, err := models.CheckUserExistence(db, 1, gEmail)
	if err != nil {
		panic(err)
	}

	if !userVerificationExist {
		JsonResponse(c, CodeOK, "", nil)
		return
	}

	// check if the verification code is correct
	result := models.UserVerification{}
	err = models.GetUserByEmail(db, 1, gEmail, &result)
	if err != nil {
		panic(err)
	}
	if result.VerificationCode != gVerificationCode {
		JsonResponse(c, CodeAuthError, "", nil)
		return
	}

	// insert to User table
	hash, err := models.HashPassword(gPassword)
	if err != nil {
		panic(err)
	}

	userId := bson.NewObjectId()

	err = models.GetUserCollection(db, 0).Insert(&models.User{
		Id: 		 userId,
		Email:       gEmail,
		Password:    hash,
		TimeCreated: time.Now(),
	})

	if err != nil {
		panic(err)
	}

	err = models.NotificationAddEmail(db, userId, gEmail, "--default--")

	if err != nil {
		panic(err)
	}

	JsonResponse(c, CodeOK, "", nil)
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

