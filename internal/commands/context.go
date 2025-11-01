package commands

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"

	"serverbot/internal/app"
	"serverbot/internal/system"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Context wraps all the information required to process a command.
type Context struct {
	AppConfig app.Config
	Runner    system.Runner
	Logger    *log.Logger

	RequestContext context.Context
	Bot            *tgbotapi.BotAPI
	Update         tgbotapi.Update
	Command        string
	Arguments      string
}

// Args returns the raw arguments string.
func (c *Context) Args() string {
	return c.Arguments
}

// ArgsList returns the arguments split by whitespace.
func (c *Context) ArgsList() []string {
	if c.Arguments == "" {
		return nil
	}
	fields := strings.Fields(c.Arguments)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// Reply sends a plain text message to the chat.
func (c *Context) Reply(text string) error {
	if c.Bot == nil || c.Update.Message == nil {
		return fmt.Errorf("cannot reply without message context")
	}

	_, err := c.ReplyMessage(text)
	return err
}

// ReplyMessage sends a plain text message and returns the Telegram response.
func (c *Context) ReplyMessage(text string) (tgbotapi.Message, error) {
	if c.Bot == nil || c.Update.Message == nil {
		return tgbotapi.Message{}, fmt.Errorf("cannot reply without message context")
	}

	msg := tgbotapi.NewMessage(c.Update.Message.Chat.ID, text)
	sent, err := c.Bot.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("send reply: %w", err)
	}
	return sent, nil
}

// ReplyHTML sends a HTML-formatted message, escaping content if requested.
func (c *Context) ReplyHTML(text string, escape bool) error {
	if c.Bot == nil || c.Update.Message == nil {
		return fmt.Errorf("cannot reply without message context")
	}

	body := text
	if escape {
		body = html.EscapeString(text)
	}

	msg := tgbotapi.NewMessage(c.Update.Message.Chat.ID, body)
	msg.ParseMode = "HTML"
	if _, err := c.Bot.Send(msg); err != nil {
		return fmt.Errorf("send html reply: %w", err)
	}
	return nil
}

// ReplyPre sends the provided content inside a <pre> block after escaping it.
func (c *Context) ReplyPre(content string) error {
	body := fmt.Sprintf("<pre>%s</pre>", html.EscapeString(strings.TrimSpace(content)))
	return c.ReplyHTML(body, false)
}

// ReplyAndEdit posts a placeholder message and edits it with the final HTML.
func (c *Context) ReplyAndEdit(initial string, finalHTML string) error {
	if c.Bot == nil || c.Update.Message == nil {
		return fmt.Errorf("cannot reply without message context")
	}

	sent, err := c.ReplyMessage(initial)
	if err != nil {
		return fmt.Errorf("send placeholder: %w", err)
	}

	return c.EditHTML(sent.MessageID, finalHTML)
}

// EditHTML edits a previously sent message using HTML formatting.
func (c *Context) EditHTML(messageID int, htmlBody string) error {
	if c.Bot == nil || c.Update.Message == nil {
		return fmt.Errorf("cannot edit without message context")
	}

	edit := tgbotapi.NewEditMessageText(c.Update.Message.Chat.ID, messageID, htmlBody)
	edit.ParseMode = "HTML"
	if _, err := c.Bot.Send(edit); err != nil {
		return fmt.Errorf("edit message: %w", err)
	}
	return nil
}

// ReplyError sends a user-facing error message and logs the underlying error.
func (c *Context) ReplyError(userMessage string, err error) error {
	if err != nil && c.Logger != nil {
		c.Logger.Printf("command %s failed: %v", c.Command, err)
	}
	return c.Reply(userMessage)
}

// IsAdmin reports whether the message originates from the admin chat.
func (c *Context) IsAdmin() bool {
	if c.Update.Message == nil {
		return false
	}
	return c.Update.Message.Chat.ID == c.AppConfig.AdminID
}
