---
title: "一个oauth示例"
date: 2024-01-23T13:30:50+08:00
---

oauth2的本质就是你想进一家工厂, 但是门卫说你没证件你不能进, 你说我有隔壁厂子的证件, 你们都认识, 可以不, 他说也行, 于是根据你隔壁 厂的信息, 给你创建了一个`隔壁厂_用户1` 的证件, 网上讲解的文章很多了, 这里就直接给一份示例代码吧

目录结构

```bash
.
├── README.md
├── cmd
│   ├── client
│   │   └── main.go
│   └── server
│       └── main.go
├── go.mod
├── go.sum
├── gorm.db
├── public
│   ├── a-index.html
│   ├── login.html
│   └── reg.html
└── 表.sql
```

### client/main.go

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/phuslu/log"
	"golang.org/x/oauth2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"net/http"
	"time"
)

func init() {
	log.DefaultLogger = log.Logger{
		TimeFormat: "15:04:05",
		Caller:     1,
		Writer: &log.ConsoleWriter{
			ColorOutput:    true,
			QuoteString:    true,
			EndWithMessage: true,
		},
	}
}

const (
	authServerURL = "http://oauth.inksnw.com"
)

var (
	MyOAuth = oauth2.Config{
		ClientID:     "clienta",
		ClientSecret: "123",
		Scopes:       []string{"all"},
		RedirectURL:  "http://localhost:8080/getcode",
		Endpoint: oauth2.Endpoint{
			AuthURL:  authServerURL + "/auth",  //获取授权码 地址
			TokenURL: authServerURL + "/token", //获取token地址
		},
	}
)

func main() {
	log.Info().Msg("start server")
	gin.SetMode(gin.ReleaseMode)
	InitDB()
	r := gin.Default()
	r.LoadHTMLGlob("public/*")
	loginUrl := MyOAuth.AuthCodeURL("fake_state")

	r.GET("/", func(c *gin.Context) {
		log.Info().Msg("请求首页")
		c.HTML(200, "a-index.html", map[string]string{
			"loginUrl": loginUrl,
		})
	})
	r.GET("/user/reg", func(c *gin.Context) {
		data := map[string]string{"error": "", "message": ""}
		c.HTML(200, "reg.html", data)
	})

	//注册用户
	r.POST("/user/reg", func(c *gin.Context) {
		data := map[string]string{"error": "", "message": ""}

		passport := GetUserInfo(authServerURL+"/info", c.Query("token"))
		
		expr := time.Unix(passport.Expire, 0).Format("2006-01-02 15:04:05")
		log.Info().Msgf("获取第三方用户信息: 用户id: %s, 过期时间: %s", passport.UserId, expr)

		user, err := AddNewUser(
			c.PostForm("userName"),
			c.PostForm("userPass"),
			passport.UserId,
			c.Query("source"))
		log.Info().Msgf("注册用户: 用户名: %s,  第三方用户id: %s", c.PostForm("userName"), passport.UserId)

		if err != nil {
			data["error"] = err.Error()
			log.Error().Msgf(err.Error())
			c.HTML(200, "reg.html", data)
			return
		}
		if passport.UserId != "" {
			data["message"] = fmt.Sprintf("账号绑定成功,您的用户名是: %s,第三方账号是: %s", user.UserName, passport.UserId)
		} else {
			data["message"] = fmt.Sprintf("账号创建成功,您的用户名是: %s", user.UserName)
		}

		c.HTML(200, "reg.html", data)
	})

	r.GET("/getcode", func(c *gin.Context) {
		source := "inksnw"            //来源
		code, _ := c.GetQuery("code") //得到的授权码
		log.Info().Msgf("请求token: 授权码: %s", code)
		token, err := MyOAuth.Exchange(c, code)
		if err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
			return
		}
		passport := GetUserInfo(authServerURL+"/info", token.AccessToken)
		user := GetUserName(source, passport.UserId)
		log.Info().Msgf("查询第三方用户信息: 用户名: %s", user.UserName)
		if user.UserName != "" {
			c.String(200, "oauth验证成功, 您在本站的用户名是:%s", user.UserName)
			return
		}
		log.Info().Msgf("用户不存在,跳转注册页面: 用户名: %s", user.UserName)
		location := fmt.Sprintf("/user/reg?token=%s&source=%s", token.AccessToken, source)
		c.Redirect(302, location)
	})

	r.Run(":8080")
}

var Gorm *gorm.DB

func InitDB() {
	Gorm = gormDB()
}
func gormDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
	if err != nil {
		log.Fatal().Msgf("gorm open error:%s", err.Error())
	}
	myDB, err := db.DB()
	if err != nil {
		log.Fatal().Msgf("gorm db error:%s", err.Error())
	}
	myDB.SetMaxIdleConns(5)
	myDB.SetMaxOpenConns(10)
	myDB.SetConnMaxLifetime(time.Second * 30)
	return db
}

type UserModel struct {
	UserId       int    `gorm:"column:user_id"`
	UserName     string `gorm:"column:user_name"`
	UserPwd      string `gorm:"column:user_pwd"`
	SourceId     int    `gorm:"column:source_id"`
	SourceUserId string `gorm:"column:source_userid"`
}

type Source struct {
	SourceId   int    `gorm:"column:source_id"`
	SourceName string `gorm:"column:source_name"`
}

func AddNewUser(userName string, pwd string, userID string, sourceName string) (*UserModel, error) {

	source := &Source{}
	if sourceName != "" {
		if err := Gorm.Table("sources").Where("source_name=?", sourceName).First(source).Error; err != nil {
			return nil, fmt.Errorf("来源不合法:%s", err.Error())
		}
	}
	user := &UserModel{UserName: userName, UserPwd: pwd, SourceId: source.SourceId, SourceUserId: userID}
	if err := Gorm.Table("users").Create(user).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败:%s", err.Error())
	}
	return user, nil
}

func GetUserName(sourceName string, sourceUserID string) *UserModel {
	source := &Source{}
	if err := Gorm.Table("sources").Where("source_name=?", sourceName).First(source).Error; err != nil {
		panic(fmt.Errorf("error source:%s", err.Error()))
	}
	userModel := &UserModel{}
	err := Gorm.Table("users").Where("source_id=? and source_userid=?",
		source.SourceId, sourceUserID).First(userModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return userModel
		}
		panic(err)
	}

	return userModel
}

type OAuthUser struct {
	UserId string `json:"user_id"`
	Expire int64  `json:"expire"`
}

func GetUserInfo(url string, token string) *OAuthUser {
	var req *http.Request
	var err error
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+token)

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err.Error())
	}
	defer rsp.Body.Close()
	b, _ := io.ReadAll(rsp.Body)
	oauthUser := &OAuthUser{}
	err = json.Unmarshal(b, oauthUser)
	if err != nil {
		panic(err.Error())
	}
	log.Info().Msgf("获取第三方用户信息: 用户id: %s, 过期时间: %d", oauthUser.UserId, oauthUser.Expire)
	return oauthUser
}

```

## server/main.go

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/gorilla/sessions"
	"github.com/phuslu/log"
	"net/http"
	"time"
)

var sessionStore = sessions.NewCookieStore([]byte("123456"))

func init() {
	sessionStore.Options.Domain = "oauth.inksnw.com"
	sessionStore.Options.Path = "/"
	sessionStore.Options.MaxAge = 0 //关掉浏览器就清掉session
	log.DefaultLogger = log.Logger{
		TimeFormat: "15:04:05",
		Caller:     1,
		Writer: &log.ConsoleWriter{
			ColorOutput:    true,
			QuoteString:    true,
			EndWithMessage: true,
		},
	}
}

func main() {
	log.Info().Msg("start server")
	manager := manage.NewDefaultManager()
	manager.MustTokenStorage(store.NewMemoryTokenStore())

	clientStore := store.NewClientStore()
	err := clientStore.Set("clienta", &models.Client{
		ID:     "clienta",
		Secret: "123",
		Domain: "http://localhost:8080",
	})
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	manager.MapClientStorage(clientStore)

	srv := server.NewDefaultServer(manager)
	srv.SetUserAuthorizationHandler(userAuthorizeHandler)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	//响应授权码
	r.GET("/auth", func(context *gin.Context) {
		err := srv.HandleAuthorizeRequest(context.Writer, context.Request)
		log.Info().Msg("收到申请授权码请求")
		if err != nil {
			panic(err.Error())
		}
	})
	//根据授权码获取token
	r.POST("/token", func(context *gin.Context) {
		err := srv.HandleTokenRequest(context.Writer, context.Request)
		log.Info().Msg("收到申请token请求")
		if err != nil {
			panic(err.Error())
		}
	})
	r.GET("/login", func(c *gin.Context) {
		data := map[string]string{"error": ""}
		log.Info().Msg("打开第三方登录页面")
		c.HTML(200, "login.html", data)
	})

	r.POST("/login", func(c *gin.Context) {
		data := map[string]string{"error": ""}

		uname, upass := c.PostForm("userName"), c.PostForm("userPass")
		if uname+upass == "admin123" {
			s, err := sessionStore.Get(c.Request, "LoginUser")
			if err != nil {
				panic(err.Error())
			}
			s.Values["userID"] = uname
			err = s.Save(c.Request, c.Writer) //save 保存
			log.Info().Msgf("用户登录: 用户名: %s, 重定向到auth接口", uname)
			c.Redirect(302, "/auth?"+c.Request.URL.RawQuery)
			return
		}
		log.Info().Msgf("用户登录: 用户名: %s, 密码: %s, 登录失败", uname, upass)

		data["error"] = "用户名密码错误"
		c.HTML(200, "login.html", data)
	})

	r.GET("/info", func(context *gin.Context) {
		token, err := srv.ValidationBearerToken(context.Request)
		if err != nil {
			panic(err.Error())
		}
		log.Info().Msg("收到获取用户信息请求")
		ret := gin.H{
			"user_id": token.GetUserID(),
			"expire":  int64(token.GetAccessCreateAt().Add(token.GetAccessExpiresIn()).Sub(time.Now()).Seconds()),
		}
		context.JSON(200, ret)
	})

	r.LoadHTMLGlob("public/*.html")
	r.Run(":80")

}
func userAuthorizeHandler(w http.ResponseWriter, r *http.Request) (userID string, err error) {
	s, err := sessionStore.Get(r, "LoginUser")
	if err != nil {
		return userID, err
	}
	userID, ok := s.Values["userID"].(string)
	if !ok || userID == "" {
		log.Info().Msgf("session无记录, 用户未登录,跳转到登录接口")
		w.Header().Set("Location", "/login?"+r.URL.RawQuery)
		w.WriteHeader(302)
	}
	return userID, nil
}

```

### public前端文件

a-index.html

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Title</title>
</head>
<body>
<div class="body">
    <a href="{{.loginUrl}}">使用xx账号登录</a>
</div>
</body>
</html>
```

login.html

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Title</title>
</head>
<body>
<div class="body">
    <form method="post">
        <div>
            <span>用户名:</span>
            <label>
                <input name="userName" type="text"/>
            </label>
        </div>
        <div>
            <span>密码:</span>
            <label>
                <input name="userPass" type="password"/>
            </label>
        </div>
        <div><input type="submit" value="登录"/></div>
        <div style="color: red">{{.error}}</div>
    </form>
</div>
</body>
</html>
```

reg.html

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>注册用户</title>
</head>
<body>
<div class="body">
    <form method="post">
        <div>
            <span>用户名:</span>
            <label>
                <input name="userName" type="text"/>
            </label>
        </div>
        <div>
            <span>密码:</span>
            <label>
                <input name="userPass" type="password"/>
            </label>
        </div>
        <div><input type="submit" value="注册"/></div>
        <div>{{.message}}</div>
        <div style="color: red">{{.error}}</div>
    </form>
</div>
</body>
</html>
```

### 数据库

```sql
CREATE TABLE `sources`
(
    `source_id`   INTEGER PRIMARY KEY AUTOINCREMENT,
    `source_name` TEXT,
    `source_uri`  TEXT
);

INSERT INTO `sources` (`source_id`, `source_name`, `source_uri`) VALUES (1, 'inksnw', 'http://oauth.inksnw.com');
INSERT INTO `sources` (`source_id`, `source_name`, `source_uri`) VALUES (2, 'github', 'http://github.com');


CREATE TABLE `users`
(
    `user_id`       INTEGER PRIMARY KEY AUTOINCREMENT,
    `user_name`     TEXT,
    `user_pwd`      TEXT,
    `user_addtime`  DATETIME DEFAULT CURRENT_TIMESTAMP,
    `source_id`     INTEGER,
    `source_userid` TEXT,
    UNIQUE(`user_name`),
    UNIQUE(`source_id`, `source_userid`)
);
```



