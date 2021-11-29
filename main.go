package main

import (
	"encoding/json"
	"log"
	"net/url"
	"github.com/gotify/plugin-api"
	"os/signal"
	"github.com/gorilla/websocket"
	"os"
	"time"
	"net/smtp"
	"github.com/jordan-wright/email"
	"errors"
	"gopkg.in/go-playground/validator.v9"
	"github.com/spf13/viper"
	"github.com/fsnotify/fsnotify"
)

func start() (err error){
	//Get viper
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath("./data")
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Println("Config file changed:", e.Name)
		log.Println("Config file changed:", e.Name)
		if err = v.ReadInConfig(); err != nil {
			log.Printf("couldn't load config: %s", err)
			//os.Exit(1)
			return
		}
		if err = v.Unmarshal(&config); err != nil {
			log.Printf("couldn't read config: %s", err)
			return
		}
	})

	if err = v.ReadInConfig(); err != nil {
		log.Printf("couldn't load config %s", err)
		return  err
	}


	if err = v.Unmarshal(&config ); err != nil {
		log.Printf("couldn't read config: %s", err)
		return  err
	}
	return nil
}
//////////////////////////////////////////////////////////////////////////////////////////////////////
type (
	// EmailConf : Go email settings for incoming and outgoing messages.
	EmailConf struct{
		Identity  string  `json:"identity"`
		UserName  string  `json:"username"`
		Password  string  `json:"password"`
		Host      string  `json:"host"`
		Port      string  `json:"port"`
		Address   string  `json:"address"`
		FromEmail string  `json:"fromemail"`
	}

	EmailBody struct {
		Email   string `json:"email" form:"email" query:"email" validate:"required,email"`
		Subject string `json:"subject" form:"subject" query:"subject" validate:"required"`
		Message string `json:"message" form:"message" query:"message" validate:"required"`
	}

	MyConfig struct {
		Mailer  EmailConf   `mapstructure:"emailconfig"`
	}
)

var (
	config MyConfig
)

/////////////////////////////////////////////////////////////////////////////////////////////////////////
// GetGotifyPluginInfo returns gotify plugin info.
func GetGotifyPluginInfo() plugin.Info {
	return plugin.Info{
		ModulePath:  "github.com/elgonlabs/gotify-emailer",
		Author:      "Edwin S.",
		Website:     "",
		Description: "Emailer for Gotify",
		Name:        "Gotify-Emailer",
	}
}

type User struct {
	Email string `json:"email" validate:"required,email"`
}

// EmailerPlugin is the gotify plugin instance.
type EmailerPlugin struct {
	enabled        bool
	msgHandler     plugin.MessageHandler
	storageHandler plugin.StorageHandler
	config         *Config
	basePath       string
	host           string
	scheme         string
}

// SetStorageHandler implements plugin.Storager
func (c *EmailerPlugin) SetStorageHandler(h plugin.StorageHandler) {
	c.storageHandler = h
}

// SetMessageHandler implements plugin.Messenger.
func (c *EmailerPlugin) SetMessageHandler(h plugin.MessageHandler) {
	c.msgHandler = h
}

// Storage defines the plugin storage scheme
type Storage struct {
	CalledTimes int `json:"called_times"`
}

// Config defines the plugin config scheme

type Config struct {
	Email        string       `yaml:"email" validate:"required"`
	ClientToken  string       `yaml:"client_token" validate:"required"`
	HostServer   string       `yaml:"host_server" validate:"required"`
}


// DefaultConfig implements plugin.Configurer
func (c *EmailerPlugin) DefaultConfig() interface{} {
	config := &Config{
		Email: "",
		ClientToken: "",
		HostServer: "ws://localhost:8000",
	}
	return config
}

// ValidateAndSetConfig implements plugin.Configurer
func (c *EmailerPlugin) ValidateAndSetConfig(conf interface{}) error {
	config := conf.(*Config)
	//validate
	v := validator.New()
	err := v.Struct(config)
	if err != nil{
		log.Println(err.Error())
		return  errors.New("Validation errors : "+ err.Error())
	}
	////////////////////
	c.config = config
	return nil
}

// Enable enables the plugin.
func (c *EmailerPlugin) Enable() error {
	//fmt.Println("scheme server is : ", c.scheme)
	if len (c.config.HostServer) < 1 {
		return errors.New("please enter the correct web server")
	}else{
		//check if token exixts
		if len (c.config.ClientToken) < 1 { //if empty
			return errors.New("please add the client token first")
		}

		//create urlmyurl := c.config.HostServer + "/stream?token=" + c.config.ClientToken
		myurl := c.config.HostServer + "/stream?token=" + c.config.ClientToken
		//check if it valid

		err := c.TestSocket(myurl)
		if err != nil{
			return errors.New("web server url is not valid, either client_token or url is not valid")
		}
	}

	///////////////////////////////////////////////////
	if len (c.config.Email) < 1 {
		return errors.New("please add an email first")
	}else{
		//check if it is valid
		v := validator.New()
		a := User{
			Email: c.config.Email,
		}
		err := v.Struct(a)
		if err != nil{
			log.Println(err.Error())
			return  errors.New("Email not valid :"+ err.Error())
		}
	}

	//////////////////////////////////////////////
	//create url
	myurl := c.config.HostServer + "/stream?token=" + c.config.ClientToken
	log.Println("Websocket url : ", myurl)
	///////////////////////////
	go c.ReadMessages(myurl, c.config.Email)
	////////////////////////////////////////
	c.enabled = true
	log.Println("Mailer plugin enabled")
	////////////////////////////////////
	return nil
}

// Disable disables the plugin.
func (c *EmailerPlugin) Disable() error {
	c.enabled = false
	log.Println("Mailer plugin disabled")
	return nil
}

// GetDisplay implements plugin.Displayer.
func (c *EmailerPlugin) GetDisplay(location *url.URL) string {

	message := `
	HOW TO FILL THE CONFIGURER

	1. Enter the email address needed to receive all the messages.
	2. Create a client from the clients area to get the client token.
	3. The default host server is "ws://localhost:8000".

	NB: Please re-enable plugin after making changes.
	`
	return message
}

// NewGotifyPluginInstance creates a plugin instance for a user context.
func NewGotifyPluginInstance(ctx plugin.UserContext) plugin.Plugin {
	return &EmailerPlugin{}
}

func (emailer *EmailerPlugin) TestSocket(myurl string) (err error){
	_, _, err = websocket.DefaultDialer.Dial(myurl, nil)
	if err != nil {
		log.Println("Test dial error : ", err)
		return  err
	}
	return nil
}

func (c *EmailerPlugin) SendEmail(msg plugin.Message, toemail string) (err error) {
	err = start()
	if err != nil{
		return
	}

	e := email.NewEmail()
	e.From = config.Mailer.UserName
	//toemail := emailer.config.Email
	e.To = []string{toemail}
	e.Subject = msg.Title
	e.Text = []byte(msg.Message)
	//e.HTML = []byte("<h1>Fancy HTML is supported, too!</h1>")
	err = e.Send(config.Mailer.Address, smtp.PlainAuth("", config.Mailer.UserName, config.Mailer.Password, config.Mailer.Host))
	return
}

func (emailer *EmailerPlugin) ReadMessages(myurl, toemail string) (err error){
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	c, _, err := websocket.DefaultDialer.Dial(myurl, nil)
	if err != nil {
		log.Fatal("Dial error : ", err)
		return  err
	}
	log.Printf("Connected to %s", myurl)
	defer c.Close()
	///////////////////////////////////////
	done := make(chan struct{})
	/////////////////////////////
	msg := plugin.Message{}
	////////////////////////////////////
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Fatal("Websocket read message error :", err)
				return
			}
			///////////////////////////////
			if err := json.Unmarshal(message, &msg); err != nil {
				//panic(err)
				log.Fatal("Json Unmarshal error :", err)
				return
			}
			//send email
			err = emailer.SendEmail(msg, toemail)
			if err != nil{
				log.Printf("Email error : %v ", err)
			}

		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				log.Println("write:", err)
				return err
				//log.Fatal("Websocket write message error :", err)
			}
		case <-interrupt:
			log.Println("interrupt")

		// Cleanly close the connection by sending a close message and then
		// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return err
			}
				select {
				case <-done:
				case <-time.After(time.Second):
				}
			return err
		}
	}

}

func main() {
	panic("this should be built as go plugin")
}

