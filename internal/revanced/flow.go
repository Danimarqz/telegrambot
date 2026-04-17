package revanced

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"serverbot/internal/commands"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const buildIdleTimeout = time.Minute

// Service holds the dependencies for the revanced build pipeline.
type Service struct {
	Store    *StateStore
	RepoDir  string
	ServeDir string
	BaseURL  string
	Logger   *log.Logger

	timerMu sync.Mutex
	timer   *time.Timer
}

// NewService creates a ready-to-use Service.
func NewService(stateFile, repoDir, serveDir, baseURL string, logger *log.Logger) *Service {
	return &Service{
		Store:    NewStateStore(stateFile),
		RepoDir:  repoDir,
		ServeDir: serveDir,
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Logger:   logger,
	}
}

// HandleBuild is the handler for /revanced_build.
func (s *Service) HandleBuild(ctx *commands.Context) error {
	st, err := s.Store.Load()
	if err != nil {
		return ctx.ReplyError("Error al leer estado", err)
	}
	if st.Phase != PhaseIdle {
		return ctx.Reply(fmt.Sprintf("Pipeline ocupado (fase: %s). Usa /revanced_cancel para reiniciar.", st.Phase))
	}

	if err := s.Store.Save(State{
		Phase:     PhaseResolving,
		ChatID:    ctx.Update.Message.Chat.ID,
		StartedAt: time.Now(),
	}); err != nil {
		return ctx.ReplyError("Error al guardar estado", err)
	}

	sent, err := ctx.ReplyMessage("Resolviendo versiones requeridas...")
	if err != nil {
		return err
	}

	go s.runResolve(ctx.RequestContext, ctx.Bot, ctx.Update.Message.Chat.ID, sent.MessageID)
	return nil
}

// HandleStatus is the handler for /revanced_status.
func (s *Service) HandleStatus(ctx *commands.Context) error {
	st, err := s.Store.Load()
	if err != nil {
		return ctx.ReplyError("Error al leer estado", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>Fase:</b> %s\n", html.EscapeString(string(st.Phase))))

	if !st.StartedAt.IsZero() {
		b.WriteString(fmt.Sprintf("<b>Inicio:</b> %s\n", st.StartedAt.Format("15:04:05")))
	}
	if st.Error != "" {
		b.WriteString(fmt.Sprintf("<b>Error:</b> %s\n", html.EscapeString(st.Error)))
	}

	for _, apk := range st.RequiredAPKs {
		status := "⏳"
		if apk.Received {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <code>%s</code> v%s\n", status, html.EscapeString(apk.PackageName), html.EscapeString(apk.Version)))
	}

	return ctx.ReplyHTML(b.String(), false)
}

// HandleCancel is the handler for /revanced_cancel.
func (s *Service) HandleCancel(ctx *commands.Context) error {
	s.stopTimer()
	if err := s.Store.Save(State{Phase: PhaseIdle}); err != nil {
		return ctx.ReplyError("Error al resetear estado", err)
	}
	return ctx.Reply("Pipeline reiniciado a idle.")
}

// armTimer (re)starts the idle timer.  When it fires, a build is launched
// with whichever APKs have been received so far.
func (s *Service) armTimer(ctx context.Context, bot *tgbotapi.BotAPI, chatID int64) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(buildIdleTimeout, func() {
		s.onBuildTimeout(ctx, bot, chatID)
	})
}

func (s *Service) stopTimer() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *Service) onBuildTimeout(ctx context.Context, bot *tgbotapi.BotAPI, chatID int64) {
	st, err := s.Store.Load()
	if err != nil || st.Phase != PhaseAwaitingAPK {
		return
	}

	received := receivedAppNames(st.RequiredAPKs)
	if len(received) == 0 {
		_ = s.Store.Save(State{Phase: PhaseIdle})
		_ = sendText(bot, chatID, "Timeout, no recibi ningun APK. Pipeline vuelve a idle.")
		return
	}

	_ = sendText(bot, chatID, fmt.Sprintf("60s sin APK nuevo. Arranco build con: %s", strings.Join(received, ", ")))
	if err := s.Store.Update(func(state *State) error {
		state.Phase = PhaseBuilding
		return nil
	}); err != nil {
		s.log("update state error: %v", err)
	}
	go s.runBuild(ctx, bot, chatID)
}

// HandleDocument processes an incoming document when the pipeline is
// awaiting APKs.  Returns true if the document was consumed.
func (s *Service) HandleDocument(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update, logger *log.Logger) bool {
	if update.Message == nil || update.Message.Document == nil {
		return false
	}

	st, err := s.Store.Load()
	if err != nil || st.Phase != PhaseAwaitingAPK {
		return false
	}

	doc := update.Message.Document
	if !strings.HasSuffix(strings.ToLower(doc.FileName), ".apk") {
		return false
	}

	// Download file from the Telegram API (works with local sidecar).
	fileConfig := tgbotapi.FileConfig{FileID: doc.FileID}
	tgFile, err := bot.GetFile(fileConfig)
	if err != nil {
		s.log("getFile error: %v", err)
		return false
	}

	// When using a local bot-api sidecar the FilePath is an absolute local
	// path.  Otherwise fall back to HTTP download.
	var localPath string
	if filepath.IsAbs(tgFile.FilePath) {
		localPath = tgFile.FilePath
	} else {
		tmp, dlErr := downloadToTemp(tgFile.Link(bot.Token))
		if dlErr != nil {
			s.log("download error: %v", dlErr)
			_ = sendText(bot, update.Message.Chat.ID, fmt.Sprintf("Error al descargar APK: %s", dlErr))
			return true
		}
		defer os.Remove(tmp)
		localPath = tmp
	}

	info, err := ReadAPKInfo(localPath)
	if err != nil {
		s.log("readAPK error: %v", err)
		_ = sendText(bot, update.Message.Chat.ID, fmt.Sprintf("No se pudo leer el APK: %s", err))
		return true
	}

	matched := false
	allReceived := true
	if err := s.Store.Update(func(state *State) error {
		for i := range state.RequiredAPKs {
			ra := &state.RequiredAPKs[i]
			if ra.PackageName == info.PackageName && ra.Version == info.VersionName {
				// Copy APK using app_name (e.g. youtube.apk), not package_name,
				// because the Python builder expects <app_name>.apk.
				dst := filepath.Join(s.RepoDir, "apks", ra.AppName+".apk")
				if cpErr := copyFile(localPath, dst); cpErr != nil {
					return fmt.Errorf("copiar APK: %w", cpErr)
				}
				ra.Received = true
				matched = true
			}
			if !ra.Received {
				allReceived = false
			}
		}
		return nil
	}); err != nil {
		s.log("update state error: %v", err)
		_ = sendText(bot, update.Message.Chat.ID, fmt.Sprintf("Error interno: %s", err))
		return true
	}

	if !matched {
		_ = sendText(bot, update.Message.Chat.ID,
			fmt.Sprintf("APK <code>%s</code> v%s no coincide con ninguno requerido.",
				html.EscapeString(info.PackageName), html.EscapeString(info.VersionName)))
		return true
	}

	if allReceived {
		s.stopTimer()
		_ = sendText(bot, update.Message.Chat.ID, "Todos los APKs recibidos. Iniciando build...")
		if err := s.Store.Update(func(state *State) error {
			state.Phase = PhaseBuilding
			return nil
		}); err != nil {
			s.log("update state error: %v", err)
		}
		go s.runBuild(ctx, bot, update.Message.Chat.ID)
	} else {
		s.armTimer(ctx, bot, update.Message.Chat.ID)
		_ = sendText(bot, update.Message.Chat.ID,
			fmt.Sprintf("APK <code>%s</code> v%s recibido. (60s para build parcial)",
				html.EscapeString(info.PackageName), html.EscapeString(info.VersionName)))
	}

	return true
}

// runResolve executes the resolver in the background and transitions to
// awaiting_apks or reports the error.
func (s *Service) runResolve(ctx context.Context, bot *tgbotapi.BotAPI, chatID int64, editMsgID int) {
	versions, err := Resolve(ctx, s.RepoDir)
	if err != nil {
		s.log("resolve error: %v", err)
		_ = s.Store.Save(State{Phase: PhaseIdle, Error: err.Error()})
		_ = editText(bot, chatID, editMsgID, fmt.Sprintf("Error en resolución:\n<pre>%s</pre>", html.EscapeString(err.Error())))
		return
	}

	// Check if serve dir already has patched APKs matching the resolved versions.
	// If so, skip the entire build and send download links immediately.
	if s.ServeDir != "" {
		resolvedAPKs := make([]RequiredAPK, len(versions))
		for i, v := range versions {
			resolvedAPKs[i] = RequiredAPK{
				PackageName: v.PackageName,
				AppName:     v.AppName,
				Version:     v.Version,
				Received:    true,
			}
		}
		if names := s.alreadyPublished(resolvedAPKs); len(names) > 0 {
			_ = s.Store.Save(State{Phase: PhaseIdle})
			_ = editText(bot, chatID, editMsgID, s.formatPublished("Ya publicado (misma version)", names))
			return
		}
	}

	// Determine which APKs are already present vs. needed.
	var required []RequiredAPK
	var present []string
	for _, v := range versions {
		apkPath := filepath.Join(s.RepoDir, "apks", sanitizeFilename(v.PackageName)+".apk")
		if _, err := os.Stat(apkPath); err == nil {
			// Check if existing APK matches the required version.
			info, infoErr := ReadAPKInfo(apkPath)
			if infoErr == nil && info.VersionName == v.Version {
				present = append(present, fmt.Sprintf("%s v%s", v.AppName, v.Version))
				required = append(required, RequiredAPK{
					PackageName: v.PackageName,
					AppName:     v.AppName,
					Version:     v.Version,
					Received:    true,
				})
				continue
			}
		}
		required = append(required, RequiredAPK{
			PackageName: v.PackageName,
			AppName:     v.AppName,
			Version:     v.Version,
		})
	}

	// Check if all APKs are already available.
	allReady := true
	for _, r := range required {
		if !r.Received {
			allReady = false
			break
		}
	}

	if allReady {
		_ = s.Store.Save(State{
			Phase:        PhaseBuilding,
			RequiredAPKs: required,
			ChatID:       chatID,
			StartedAt:    time.Now(),
		})
		_ = editText(bot, chatID, editMsgID, "Todos los APKs disponibles. Iniciando build...")
		go s.runBuild(ctx, bot, chatID)
		return
	}

	_ = s.Store.Save(State{
		Phase:        PhaseAwaitingAPK,
		RequiredAPKs: required,
		ChatID:       chatID,
		StartedAt:    time.Now(),
	})

	var b strings.Builder
	b.WriteString("<b>APKs requeridos:</b>\n")
	for _, r := range required {
		status := "⏳"
		if r.Received {
			status = "✅"
		}
		b.WriteString(fmt.Sprintf("%s <code>%s</code> v%s\n", status, html.EscapeString(r.PackageName), html.EscapeString(r.Version)))
	}
	if len(present) > 0 {
		b.WriteString(fmt.Sprintf("\nYa disponibles: %s\n", strings.Join(present, ", ")))
	}
	b.WriteString("\nEnvia los APKs faltantes como documentos. (60s para build parcial)")
	_ = editText(bot, chatID, editMsgID, b.String())

	s.armTimer(ctx, bot, chatID)
}

// runBuild executes the full build, publishes APKs, and notifies the chat.
// It streams container output and updates a single Telegram message with progress.
func (s *Service) runBuild(ctx context.Context, bot *tgbotapi.BotAPI, chatID int64) {
	s.stopTimer()

	st, loadErr := s.Store.Load()
	appNames := []string{}
	if loadErr == nil {
		appNames = receivedAppNames(st.RequiredAPKs)
	}
	apps := strings.Join(appNames, ",")
	total := len(appNames)

	sent, sendErr := sendTextMsg(bot, chatID, "Iniciando build...")
	if sendErr != nil {
		s.log("send error: %v", sendErr)
		return
	}
	msgID := sent.MessageID

	var (
		statusMu     sync.Mutex
		lastStatus   string
		lastEditAt   time.Time
		done         int
		editThrottle = time.Second
	)

	setStatus := func(text string) {
		statusMu.Lock()
		defer statusMu.Unlock()
		if text == lastStatus {
			return
		}
		lastStatus = text
		if time.Since(lastEditAt) < editThrottle {
			return
		}
		_ = editText(bot, chatID, msgID, text)
		lastEditAt = time.Now()
	}

	onLine := func(line string) {
		switch {
		case strings.HasPrefix(line, "Trying to build "):
			app := strings.TrimPrefix(line, "Trying to build ")
			setStatus(fmt.Sprintf("Parcheando <b>%s</b> (%d/%d)...",
				html.EscapeString(app), done+1, total))
		case strings.Contains(line, "INFO: Compiling modified resources"):
			setStatus(fmt.Sprintf("Compilando recursos (%d/%d)...", done+1, total))
		case strings.Contains(line, "INFO: Writing resource APK"):
			setStatus(fmt.Sprintf("Escribiendo APK (%d/%d)...", done+1, total))
		case strings.Contains(line, "INFO: Aligning APK"):
			setStatus(fmt.Sprintf("Alineando APK (%d/%d)...", done+1, total))
		case strings.Contains(line, "INFO: Signing APK"):
			setStatus(fmt.Sprintf("Firmando (%d/%d)...", done+1, total))
		case strings.Contains(line, "Successfully completed"):
			statusMu.Lock()
			done++
			statusMu.Unlock()
			setStatus(fmt.Sprintf("%d/%d apps parcheadas...", done, total))
		case strings.Contains(line, "FAILED"):
			setStatus(fmt.Sprintf("Fallo detectado (%d/%d)...", done, total))
		}
	}

	err := Build(ctx, s.RepoDir, apps, onLine)
	if err != nil {
		s.log("build error: %v", err)
		_ = s.Store.Save(State{Phase: PhaseIdle, Error: err.Error()})
		errMsg := err.Error()
		if len(errMsg) > 3000 {
			errMsg = errMsg[:3000] + "\n..."
		}
		_ = editText(bot, chatID, msgID, fmt.Sprintf("Build fallido:\n<pre>%s</pre>", html.EscapeString(errMsg)))
		return
	}

	published, err := Publish(s.RepoDir, s.ServeDir)
	if err != nil {
		s.log("publish error: %v", err)
		_ = s.Store.Save(State{Phase: PhaseIdle, Error: err.Error()})
		_ = editText(bot, chatID, msgID, fmt.Sprintf("Build exitoso pero error al publicar: %s", err))
		return
	}

	_ = s.Store.Save(State{Phase: PhaseIdle})
	_ = editText(bot, chatID, msgID, s.formatPublished("Build ReVanced OK", published))
}

// alreadyPublished checks if the serve dir already contains patched APKs
// whose versions match every required app.  It parses the version from the
// filename pattern Re<app_name>-Version<version>-...-output.apk instead
// of reading APK metadata (which may differ in patched APKs).
// Returns the list of filenames if all match, or nil otherwise.
func (s *Service) alreadyPublished(required []RequiredAPK) []string {
	existing, err := filepath.Glob(filepath.Join(s.ServeDir, "Re*-output.apk"))
	if err != nil || len(existing) == 0 {
		return nil
	}

	// Build a map: app_name → required version.
	want := make(map[string]string)
	for _, r := range required {
		if r.AppName != "" {
			want[strings.ToLower(r.AppName)] = r.Version
		}
	}
	if len(want) == 0 {
		return nil
	}

	// Parse each filename: Re<app>-Version<ver>-...-output.apk
	matched := 0
	var names []string
	for _, path := range existing {
		appName, ver := parseOutputFilename(filepath.Base(path))
		if appName == "" {
			continue
		}
		if reqVer, ok := want[strings.ToLower(appName)]; ok && reqVer == ver {
			matched++
			names = append(names, filepath.Base(path))
		}
	}

	// Also include MicroG if present.
	extras, _ := filepath.Glob(filepath.Join(s.ServeDir, "VancedMicroG*.apk"))
	for _, p := range extras {
		names = append(names, filepath.Base(p))
	}

	if matched < len(want) {
		return nil
	}
	return names
}

// parseOutputFilename extracts app name and version from a patched APK
// filename like "Reyoutube-Version20.45.36-PatchVersion...-output.apk".
// Returns ("youtube", "20.45.36") or ("", "") if the pattern doesn't match.
func parseOutputFilename(name string) (appName, version string) {
	// Must start with "Re" and end with "-output.apk"
	if !strings.HasPrefix(name, "Re") || !strings.HasSuffix(name, "-output.apk") {
		return "", ""
	}
	// Strip "Re" prefix
	rest := name[2:]
	// Split on "-Version" to get app name and the rest
	idx := strings.Index(rest, "-Version")
	if idx < 0 {
		return "", ""
	}
	appName = rest[:idx]
	// After "-Version", extract until next "-"
	verRest := rest[idx+len("-Version"):]
	dashIdx := strings.Index(verRest, "-")
	if dashIdx < 0 {
		return "", ""
	}
	version = verRest[:dashIdx]
	return appName, version
}

// formatPublished builds an HTML message with a title and download links.
func (s *Service) formatPublished(title string, fileNames []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s</b>\n\n", html.EscapeString(title))
	for _, name := range fileNames {
		if s.BaseURL != "" {
			fmt.Fprintf(&b, "- <a href=\"%s/%s\">%s</a>\n", s.BaseURL, name, html.EscapeString(name))
		} else {
			fmt.Fprintf(&b, "- %s\n", html.EscapeString(name))
		}
	}
	return b.String()
}

func (s *Service) log(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf("revanced: "+format, args...)
	}
}

func sendText(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, err := bot.Send(msg)
	return err
}

func sendTextMsg(bot *tgbotapi.BotAPI, chatID int64, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	return bot.Send(msg)
}

func sendHTML(bot *tgbotapi.BotAPI, chatID int64, body string) error {
	msg := tgbotapi.NewMessage(chatID, body)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true
	_, err := bot.Send(msg)
	return err
}

func editText(bot *tgbotapi.BotAPI, chatID int64, messageID int, body string) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, body)
	edit.ParseMode = "HTML"
	_, err := bot.Send(edit)
	return err
}

func sanitizeFilename(pkg string) string {
	return strings.ReplaceAll(pkg, "/", "_")
}

// receivedAppNames returns the AppName of every RequiredAPK that has been received.
func receivedAppNames(apks []RequiredAPK) []string {
	var names []string
	for _, a := range apks {
		if a.Received && a.AppName != "" {
			names = append(names, a.AppName)
		}
	}
	return names
}

func downloadToTemp(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "revanced-*.apk")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download: %w", err)
	}
	tmp.Close()
	return tmp.Name(), nil
}
