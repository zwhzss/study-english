// Wails 桌面入口：AI 英语课堂（内嵌 Angular 前端 + Go 后端服务，单文件应用）
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"study-english/server/internal/config"
)

//go:embed all:web/dist/browser
var assets embed.FS

func main() {
	cfg := config.Load()
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "AI 英语课堂",
		Width:     1280,
		Height:    860,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
			// /media/... 由 Go 从数据目录返回（Wails 页面是 wails:// 协议，file:// 跨协议会被 WebView2 拦截）
			Handler: mediaHandler(cfg.DataDir),
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatalf("桌面应用启动失败: %v", err)
	}
}

// mediaHandler 处理嵌入资源未命中的请求：
//   - /media/xxx（图片/音频/发音录音）→ 从数据目录返回文件
//   - 其他路径（SPA 路由）→ 返回 index.html
func mediaHandler(dataDir string) http.Handler {
	idx, _ := fs.ReadFile(assets, "web/dist/browser/index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/media/") {
			f := filepath.Join(dataDir, filepath.FromSlash(strings.TrimPrefix(p, "/media/")))
			http.ServeFile(w, r, f)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(idx)
	})
}
