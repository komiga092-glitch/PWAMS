package routes_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/middleware"
)

// Phase 2 CSP regression tests (§5 / §16).
//
// The application ships a strict Content-Security-Policy (script-src 'self'),
// so templates must not carry inline event handlers or inline <script>
// blocks — behavior lives in external scripts under /static/js.

func templateRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", "web", "templates"))
	if err != nil {
		t.Fatalf("failed to resolve template root: %v", err)
	}

	return root
}

func walkTemplates(t *testing.T, fn func(rel, raw string)) {
	t.Helper()

	root := templateRoot(t)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fn(filepath.ToSlash(rel), string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk templates: %v", err)
	}
}

func TestTemplates_NoInlineEventHandlers(t *testing.T) {
	for _, attr := range []string{"onclick=", "onsubmit=", "onchange=", "oninput=", "onload="} {
		walkTemplates(t, func(rel, raw string) {
			if strings.Contains(raw, attr) {
				t.Errorf("%s contains inline %q — move behavior to an external script", rel, attr)
			}
		})
	}
}

func TestTemplates_NoInlineScriptBlocks(t *testing.T) {
	walkTemplates(t, func(rel, raw string) {
		if strings.Contains(raw, "<script>") {
			t.Errorf("%s contains an inline <script> block — move it to /static/js", rel)
		}
	})
}

func TestSecurityHeaders_ScriptSrcSelfOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.SecurityHeaders())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := recorder.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header is missing")
	}

	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("script-src must be 'self', got: %s", csp)
	}

	scriptDirective := ""
	for _, directive := range strings.Split(csp, ";") {
		if strings.HasPrefix(strings.TrimSpace(directive), "script-src") {
			scriptDirective = strings.TrimSpace(directive)
		}
	}

	if strings.Contains(scriptDirective, "unsafe-inline") {
		t.Errorf("script-src must not allow unsafe-inline: %s", scriptDirective)
	}
	if strings.Contains(scriptDirective, "unsafe-eval") {
		t.Errorf("script-src must not allow unsafe-eval: %s", scriptDirective)
	}
}

func TestBaseLayout_LoadsCSPDelegatorAndApp(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(templateRoot(t), "layouts", "base.html"))
	if err != nil {
		t.Fatalf("failed to read base.html: %v", err)
	}

	html := string(raw)
	for _, asset := range []string{
		`<script src="/static/js/vendor/htmx.min.js"></script>`,
		`<script src="/static/js/app.js"></script>`,
		`<script src="/static/js/csp-delegator.js" defer></script>`,
	} {
		if !strings.Contains(html, asset) {
			t.Errorf("base.html must load %s", asset)
		}
	}
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// TestPageScripts_ReferencedByTemplates guards against orphaned page
// scripts: every page script must be referenced by a template and every
// referenced script must exist on disk.
func TestPageScripts_ReferencedByTemplates(t *testing.T) {
	referenced := map[string]bool{}

	walkTemplates(t, func(rel, raw string) {
		rest := raw
		for {
			idx := strings.Index(rest, "<script")
			if idx < 0 {
				break
			}
			end := strings.Index(rest[idx:], ">")
			if end < 0 {
				break
			}
			tag := rest[idx : idx+end+1]
			if src := between(tag, `src="`, `"`); src != "" {
				referenced[strings.TrimPrefix(src, "/static/js/")] = true
			}
			rest = rest[idx+end+1:]
		}
	})

	jsRoot, err := filepath.Abs(filepath.Join("..", "..", "web", "static", "js"))
	if err != nil {
		t.Fatalf("failed to resolve js root: %v", err)
	}

	err = filepath.Walk(jsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".js") {
			return err
		}
		if strings.Contains(path, "vendor") || strings.Contains(path, string(filepath.Separator)+"offline") {
			return nil
		}

		rel, relErr := filepath.Rel(jsRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)

		if strings.HasSuffix(rel, ".test.js") {
			return nil
		}
		if !referenced[rel] {
			t.Errorf("orphaned page script: /static/js/%s is not referenced by any template", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk js assets: %v", err)
	}

	for src := range referenced {
		path := filepath.Join(jsRoot, filepath.FromSlash(src))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("template references missing script: /static/js/%s", src)
		}
	}
}

// TestPasswordFlowPages_LoadExternalScripts pins the login/reset flow wiring
// (§4): the standalone password pages must load their extracted page scripts
// instead of inline copies.
func TestPasswordFlowPages_LoadExternalScripts(t *testing.T) {
	cases := map[string]string{
		"forgot_password.html":  "/static/js/pages/forgot_password.js",
		"verify_reset_otp.html": "/static/js/pages/verify_reset_otp.js",
		"reset_password.html":   "/static/js/pages/reset_password.js",
	}

	for name, asset := range cases {
		raw, err := os.ReadFile(filepath.Join(templateRoot(t), name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		if !strings.Contains(string(raw), `<script src="`+asset+`"`) {
			t.Errorf("%s must load %s", name, asset)
		}
	}
}

// TestCSPDelegator_CoversAllDataCspActions pins the CSP delegation contract:
// every data-csp-action emitted by a template or page script must have a
// dispatch entry in csp-delegator.js, or the control is dead in the browser.
// readJSAsset reads any static JS asset relative to web/static/js.
func readJSAsset(t *testing.T, parts ...string) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join(append([]string{"..", "..", "web", "static", "js"}, parts...)...))
	if err != nil {
		t.Fatalf("failed to resolve js asset: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read js asset %s: %v", path, err)
	}

	return string(raw)
}

func TestCSPDelegator_CoversAllDataCspActions(t *testing.T) {
	delegator := readJSAsset(t, "csp-delegator.js")

	used := map[string]bool{}
	collect := func(raw string) {
		for _, m := range regexp.MustCompile(`data-csp-action="([a-z:-]+)"`).FindAllStringSubmatch(raw, -1) {
			used[m[1]] = true
		}
	}

	walkTemplates(t, func(_, raw string) { collect(raw) })
	walkTemplTemplates(t, collect)
	walkPageScripts(t, collect)

	for action := range used {
		if !strings.Contains(delegator, `"`+action+`"`) {
			t.Errorf("data-csp-action %q has no dispatch entry in csp-delegator.js", action)
		}
	}
}

// walkTemplTemplates visits every templ source file under
// web/templates. templ components are compiled into the binary and
// rendered directly (e.g. the login page), so actions introduced there
// must be covered by the delegator too.
func walkTemplTemplates(t *testing.T, fn func(raw string)) {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", "web", "templates"))
	if err != nil {
		t.Fatalf("failed to resolve template root: %v", err)
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".templ") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk templ templates: %v", err)
	}
}

// TestPageScripts_ResolveDomIds guards the script/DOM contract: every
// getElementById target in a page script must exist as an id in some
// template, otherwise the control silently does nothing in the browser.
func TestPageScripts_ResolveDomIds(t *testing.T) {
	ids := map[string]bool{}

	walkTemplates(t, func(_, raw string) {
		for _, m := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(raw, -1) {
			ids[m[1]] = true
		}
	})

	jsRoot, err := filepath.Abs(filepath.Join("..", "..", "web", "static", "js"))
	if err != nil {
		t.Fatalf("failed to resolve js root: %v", err)
	}

	err = filepath.Walk(jsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".js") {
			return err
		}
		if strings.Contains(path, "vendor") || strings.Contains(path, string(filepath.Separator)+"offline") {
			return nil
		}
		if strings.HasSuffix(path, ".test.js") {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(jsRoot, path)

		for _, m := range regexp.MustCompile(`getElementById\("([^"]+)"\)`).FindAllStringSubmatch(string(raw), -1) {
			if !ids[m[1]] {
				t.Errorf("%s references missing DOM id %q", filepath.ToSlash(rel), m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk js assets: %v", err)
	}
}

// TestUsersPage_UserStatusControlsExcludePending pins §12: the user status
// model is Active / Disabled / Locked — Pending is a domain-workflow state,
// never a user-account status.
func TestUsersPage_UserStatusControlsExcludePending(t *testing.T) {
	html := readTemplate(t, "users.html")
	if strings.Contains(html, `value="Pending"`) {
		t.Error("users.html must not offer a Pending user status")
	}
	if !strings.Contains(html, `value="Active"`) ||
		!strings.Contains(html, `value="Disabled"`) ||
		!strings.Contains(html, `value="Locked"`) {
		t.Error("users.html status control must offer Active, Disabled and Locked")
	}

	usersJS := readPageScript(t, "users.js")
	if strings.Contains(usersJS, `"Pending"`) {
		t.Error("pages/users.js must not reference Pending as a user status")
	}
}

// walkPageScripts visits every browser-loaded script under web/static/js
// (compiled TypeScript output and handwritten page scripts alike, but not
// vendor bundles, offline service-worker modules, source maps or test
// files). Actions emitted at runtime by these scripts must be handled by
// the delegator, and the functions the delegator calls must exist there.
func walkPageScripts(t *testing.T, fn func(raw string)) {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", "web", "static", "js"))
	if err != nil {
		t.Fatalf("failed to resolve js root: %v", err)
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		if strings.Contains(path, "vendor") || strings.Contains(path, string(filepath.Separator)+"offline") {
			return nil
		}
		if strings.HasSuffix(path, ".test.js") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(string(raw))
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk page scripts: %v", err)
	}
}

// TestCSPDelegator_HandlerFunctionsExist pins the runtime dispatch contract
// (§3): every page-controller function invoked by a csp-delegator.js
// handler must be declared in a browser-loaded script — either as a
// top-level function declaration or via an explicit window.<name> =
// assignment. A missing declaration means the delegated click/submit fails
// with "X is not defined" in the browser even though every Go test passes.
func TestCSPDelegator_HandlerFunctionsExist(t *testing.T) {
	delegator := readJSAsset(t, "csp-delegator.js")

	clickBody := between(delegator, "var CLICK = {", "var SUBMIT = {")
	submitBody := between(delegator, "var SUBMIT = {", "document.addEventListener")
	if clickBody == "" || submitBody == "" {
		t.Fatal("failed to locate CLICK/SUBMIT dispatch tables in csp-delegator.js")
	}

	// Strip line comments so prose such as "// Donors page
	// (web/static/js/donors.js)" is not mistaken for a function call, and
	// the slice must end before the dispatcher bodies below the tables.
	commentRe := regexp.MustCompile(`//[^\n]*`)
	clickBody = commentRe.ReplaceAllString(clickBody, "")
	submitBody = commentRe.ReplaceAllString(submitBody, "")

	// Collect identifiers invoked inside handler bodies. Calls preceded by
	// a dot (e.g. el.closest(...)) are method calls, not page controllers.
	called := map[string]bool{}
	callRe := regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*\(`)
	ignore := map[string]bool{
		"function": true, "if": true, "for": true, "while": true,
		"switch": true, "catch": true, "return": true, "typeof": true,
		"Number": true, "String": true, "Boolean": true, "parseInt": true,
		"parseFloat": true, "isNaN": true,
	}
	for _, body := range []string{clickBody, submitBody} {
		for _, loc := range callRe.FindAllStringSubmatchIndex(body, -1) {
			name := body[loc[2]:loc[3]]
			if ignore[name] {
				continue
			}
			j := loc[0] - 1
			for j >= 0 && (body[j] == ' ' || body[j] == '\t' || body[j] == '\n' || body[j] == '\r') {
				j--
			}
			if j >= 0 && body[j] == '.' {
				continue
			}
			called[name] = true
		}
	}

	defined := map[string]bool{}
	collect := func(raw string) {
		for _, re := range []*regexp.Regexp{
			regexp.MustCompile(`function\s+([A-Za-z_$][\w$]*)\s*\(`),
			regexp.MustCompile(`window\.([A-Za-z_$][\w$]*)\s*=`),
		} {
			for _, m := range re.FindAllStringSubmatch(raw, -1) {
				defined[m[1]] = true
			}
		}
	}
	walkPageScripts(t, collect)

	for name := range called {
		if !defined[name] {
			t.Errorf("csp-delegator.js calls %s(), but no loaded page script declares function %s or window.%s — the delegated action would fail at runtime", name, name, name)
		}
	}
}

// TestDonorsPage_CSPDelegationOnly pins the donors-page migration (§4/§5):
// the Add Donor button must be dispatched through data-csp-action
// (donors:open), donors.js must expose the controller functions the
// delegator calls, and it must NOT attach direct click listeners to the
// migrated buttons (that would create duplicate handlers).
func TestDonorsPage_CSPDelegationOnly(t *testing.T) {
	html := readTemplate(t, "donors.html")

	if !strings.Contains(html, `data-csp-action="donors:open"`) {
		t.Error(`donors.html add button must carry data-csp-action="donors:open"`)
	}

	donorsJS := readJSAsset(t, "donors.js")

	for _, required := range []string{
		"window.openDonorModal = openDonorModal",
		"window.closeDonorModal = closeDonorModal",
	} {
		if !strings.Contains(donorsJS, required) {
			t.Errorf("donors.js must expose the delegator-called controller: %s", required)
		}
	}

	for _, forbidden := range []string{
		`addDonorButton?.addEventListener`,
		`closeDonorButton?.addEventListener`,
		`cancelDonorButton?.addEventListener`,
	} {
		if strings.Contains(donorsJS, forbidden) {
			t.Errorf("donors.js must not attach a direct listener for a migrated button (duplicate handler): %s", forbidden)
		}
	}

	delegator := readJSAsset(t, "csp-delegator.js")
	for _, action := range []string{"donors:open", "donors:close"} {
		if !strings.Contains(delegator, `"`+action+`": function`) {
			t.Errorf("csp-delegator.js must dispatch %q", action)
		}
	}
}

// TestStaticJS_ServedAsJavaScript pins the static asset contract (§9): the
// /static/* route used by the server (cmd/server/main.go) must serve the
// page scripts with HTTP 200, a JavaScript Content-Type and real script
// content, otherwise the browser silently drops the controllers and every
// delegated button dies.
func TestStaticJS_ServedAsJavaScript(t *testing.T) {
	gin.SetMode(gin.TestMode)

	staticRoot, err := filepath.Abs(filepath.Join("..", "..", "web", "static"))
	if err != nil {
		t.Fatalf("failed to resolve static root: %v", err)
	}

	router := gin.New()
	router.Static("/static", staticRoot)

	for _, asset := range []string{
		"/static/js/app.js",
		"/static/js/csp-delegator.js",
		"/static/js/pages/users.js",
		"/static/js/donors.js",
		"/static/js/students.js",
	} {
		req := httptest.NewRequest(http.MethodGet, asset, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d", asset, rec.Code)
			continue
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "javascript") {
			t.Errorf("GET %s: Content-Type %q must be JavaScript", asset, contentType)
		}

		if rec.Body.Len() == 0 {
			t.Errorf("GET %s: empty body", asset)
		}
	}
}
