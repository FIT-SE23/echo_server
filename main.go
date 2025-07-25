package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	storage_go "github.com/supabase-community/storage-go"
	"github.com/supabase-community/supabase-go"
)

func _jsonResponse(c echo.Context, code int, message string, returnVal interface{}) error {
	jsonData := map[string]any{
		"message": message,
		"return":  returnVal,
	}
	return c.JSON(code, jsonData)
}

func jsonResponse(c echo.Context, code int, message string, returnVal any) error {
	_, filename, line, _ := runtime.Caller(1)
	fmt.Println(filename, line)
	return _jsonResponse(c, code, message, returnVal)
}

func main() {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKEY := os.Getenv("SUPABASE_KEY")
	client, err := supabase.NewClient(supabaseURL, supabaseKEY, nil)
	if err != nil {
		fmt.Println("cannot initalize client", err)
		return
	}

	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.POST("/signup", func(c echo.Context) error {
		username := c.FormValue("username")
		email := c.FormValue("email")
		password := c.FormValue("password")
		checkSum := sha256.Sum256([]byte(password))
		hashPassword := hex.EncodeToString(checkSum[:])
		data := map[string]string{
			"username":     username,
			"email":        email,
			"hashpassword": hashPassword,
		}
		_, _, err := client.From("User").Insert(data, false, "", "", "").ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		// TODO: "return" field
		return jsonResponse(c, http.StatusOK, "", "")
	})

	e.POST("/login", func(c echo.Context) error {
		email := c.FormValue("email")
		password := c.FormValue("password")
		checkSum := sha256.Sum256([]byte(password))
		hashPassword := hex.EncodeToString(checkSum[:])
		rep, _, err := client.From("User").Select("userid", "", false).Eq("email", email).Eq("hashpassword", hashPassword).Single().ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, "Invalid email or password" /*err.Error()*/, "")
		}

		var userid map[string]string
		err = json.Unmarshal([]byte(rep), &userid)
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}
		return jsonResponse(c, http.StatusOK, "", userid)
	})
	e.GET("/user/:id", func(c echo.Context) error {
		userid := c.Param("id")

		rep, _, err := client.From("User").Select("username, email", "", false).Eq("userid", userid).Single().ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		var user map[string]any
		err = json.Unmarshal([]byte(rep), &user)
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, "Invalid userid" /*err.Error()*/, "")
		}
		return jsonResponse(c, http.StatusOK, "", user)
	})
	e.GET("/game/:id", func(c echo.Context) error {
		gameID := c.Param("id")

		// TODO: remove `publisherid`
		rep, _, err := client.From("Game").Select("publisherid, name, description, saleinformation", "", false).Eq("gameid", gameID).Single().ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}
		var gameBasicInfo map[string]any
		err = json.Unmarshal([]byte(rep), &gameBasicInfo)
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, "Invalid gameid" /*err.Error()*/, "")
		}

		rep, _, err = client.From("Game_Category").Select("*", "", false).Eq("gameid", gameID).ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}
		var gameResource []map[string]any
		err = json.Unmarshal([]byte(rep), &gameResource)
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		if len(gameResource) > 0 {
			maps.Copy(gameBasicInfo, gameResource[0])
		}
		return jsonResponse(c, http.StatusOK, "", gameBasicInfo)
	})
	e.GET("/search", func(c echo.Context) error {
		entity := c.QueryParam("entity")

		if entity == "user" {
			username := c.QueryParam("name")
			rep, _, err := client.From("User").Select("username", "", false).Like("username", "%"+username+"%").ExecuteString()
			if err != nil {
				return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
			}

			var users []map[string]any
			err = json.Unmarshal([]byte(rep), &users)
			if err != nil {
				return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
			}

			return jsonResponse(c, http.StatusOK, "", users)
		}

		return jsonResponse(c, http.StatusBadRequest, "Unsupported entity", "")
	})

	e.POST("category", func(c echo.Context) error {
		categoryName := c.FormValue("name")
		isSensitive := c.FormValue("sensitive")

		category := map[string]any{
			"categoryname": categoryName,
			"issensitive":  isSensitive != "",
		}
		_, _, err := client.From("Category").Insert(category, false, "", "", "").ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		return jsonResponse(c, http.StatusOK, "", "")
	})

	e.POST("/game", func(c echo.Context) error {
		publisherID := c.FormValue("publisherid")
		if publisherID == "" {
			return jsonResponse(c, http.StatusBadRequest, "Require publisherID", "")
		}

		// TODO: Check game's name length
		gameName := c.FormValue("gamename")
		description := c.FormValue("description")
		saleinformation := c.FormValue("saleinformation")

		game := map[string]string{
			"publisherid":     publisherID,
			"name":            gameName,
			"description":     description,
			"saleinformation": saleinformation,
		}
		_, _, err := client.From("Game").Insert(game, false, "", "gameid", "").ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}
		rep, _, err := client.From("Game").Select("gameid", "", false).Eq("name", gameName).Single().ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}
		var gameID map[string]any
		err = json.Unmarshal([]byte(rep), &gameID)
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, "Game uploaded but database does not return the id" /*err.Error()*/, "")
		}
		userID := publisherID

		// https://echo.labstack.com/docs/cookbook/file-upload
		form, err := c.MultipartForm()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		files := form.File["files"]
		errFiles := []string{}

		for _, file := range files {
			src, err := file.Open()
			if err != nil {
				errFiles = append(errFiles, file.Filename)
				// fmt.Println("Read file failed", file.Filename, err)
				continue
			}
			defer src.Close()

			upsert := true
			_, uplErr := client.Storage.UploadFile("test", userID+"/res/"+file.Filename, src, storage_go.FileOptions{Upsert: &upsert})
			if uplErr != nil {
				errFiles = append(errFiles, file.Filename)
				// fmt.Println("Upload failed", file.Filename, uplErr)
				continue
			}

			signedURL, err := client.Storage.CreateSignedUrl("test", userID+"/res/"+file.Filename, 365*24*60*60)
			if err != nil {
				errFiles = append(errFiles, file.Filename)
				// fmt.Println("Create signed url failed", file.Filename, err)
				continue
			}

			resource := map[string]string{
				"userid": userID,
				"url":    signedURL.SignedURL,
			}
			_, _, err = client.From("Resource").Insert(resource, false, "", "", "").ExecuteString()
			if err != nil {
				errFiles = append(errFiles, file.Filename)
				continue
			}

			rep, _, err := client.From("Resource").Select("resourceid", "", false).Eq("url", resource["url"]).Single().ExecuteString()
			if err != nil {
				return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
			}
			var resourceid map[string]any
			err = json.Unmarshal([]byte(rep), &resourceid)
			if err != nil {
				return jsonResponse(c, http.StatusBadRequest, "Resource uploaded but database does not return the id" /*err.Error()*/, "")
			}
			gameResource := map[string]any{
				"gameid":     gameID["gameid"],
				"resourceid": resourceid["resourceid"],
			}
			_, _, err = client.From("Game_Resource").Insert(gameResource, false, "", "", "").ExecuteString()
			if err != nil {
				errFiles = append(errFiles, file.Filename)
			}
		}

		if len(errFiles) > 0 {
			return jsonResponse(c, http.StatusBadRequest, "Upload failed. Maybe the files do not exist or have been added or cannot link gameid with resourceid", errFiles)
		}

		// TODO: categories
		rep, _, err = client.From("Category").Select("", "", false).ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}
		var knownCategories []map[string]any
		err = json.Unmarshal([]byte(rep), &knownCategories)
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		categories := strings.Split(c.FormValue("categories"), ",")
		errCat := []string{}

		for _, category := range categories {
			index := slices.IndexFunc(knownCategories, func(knownCategory map[string]any) bool {
				return knownCategory["categoryname"] == category
			})
			if index == -1 {
				errCat = append(errCat, category)
				continue
			}

			pair := map[string]any{
				"gameid":     gameID["gameid"],
				"categoryid": knownCategories[index]["categoryid"],
			}

			_, _, err = client.From("Game_Category").Insert(pair, true, "", "", "").ExecuteString()
			if err != nil {
				errCat = append(errCat, category)
				continue
			}
		}

		if len(errCat) > 0 {
			return jsonResponse(c, http.StatusBadRequest, "Unknown categories", errCat)
		}

		return jsonResponse(c, http.StatusOK, "", knownCategories)
	})

	e.POST("payment", func(c echo.Context) error {
		paymentType := c.FormValue("type")
		information := c.FormValue("information")

		payment := map[string]string{
			"type":        paymentType,
			"information": information,
		}
		_, _, err := client.From("PaymentMethod").Insert(payment, false, "", "", "").ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		return jsonResponse(c, http.StatusOK, "", "")
	})

	e.POST("publisher", func(c echo.Context) error {
		userID := c.FormValue("userid")
		paymentMethodID := c.FormValue("paymentmethodid")
		description := c.FormValue("description")

		publisher := map[string]string{
			"publisherid":     userID,
			"paymentmethodid": paymentMethodID,
			"description":     description,
		}
		_, _, err := client.From("Publisher").Insert(publisher, false, "", "", "").ExecuteString()
		if err != nil {
			return jsonResponse(c, http.StatusBadRequest, err.Error(), "")
		}

		return jsonResponse(c, http.StatusOK, "", "")
	})
	e.GET("/publisher/:id", func(c echo.Context) error {
		return jsonResponse(c, http.StatusBadRequest, "Unsupported request", "")
	})
	e.PUT("/publisher/:id", func(c echo.Context) error {
		return jsonResponse(c, http.StatusBadRequest, "Unsupported request", "")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
