package dashboard

import (
	"maps"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ValueConstraint identifies the validation applied before a token is
// projected into a CSS custom property.
type ValueConstraint string

const (
	ConstraintCSSValue          ValueConstraint = "css-value"
	ConstraintColor             ValueConstraint = "color"
	ConstraintLength            ValueConstraint = "length"
	ConstraintNonnegativeLength ValueConstraint = "nonnegative-length"
	ConstraintNumber            ValueConstraint = "number"
	ConstraintDuration          ValueConstraint = "duration"
	ConstraintEasing            ValueConstraint = "easing"
	ConstraintFontFamily        ValueConstraint = "font-family"
	ConstraintFontWeight        ValueConstraint = "font-weight"
	ConstraintShadow            ValueConstraint = "shadow"
)

// TokenStatus describes one projection, support, or consumption outcome.
type TokenStatus string

const (
	TokenResolved    TokenStatus = "resolved"
	TokenInvalid     TokenStatus = "invalid"
	TokenSupported   TokenStatus = "supported"
	TokenUnsupported TokenStatus = "unsupported"
	TokenConsumed    TokenStatus = "consumed"
	TokenUnused      TokenStatus = "unused"
)

// TokenSpec declares one supported token and its validation constraint.
type TokenSpec struct {
	Constraint ValueConstraint
	Variable   string
}

// TokenProfile declares the tokens and aliases understood by a consumer.
type TokenProfile struct {
	Name    string
	Tokens  map[string]TokenSpec
	Aliases map[string]string
}

// ProjectionOptions configures safe CSS custom-property projection.
type ProjectionOptions struct {
	Prefix  string
	Profile *TokenProfile
}

// TokenDiagnostic records one deterministic token outcome.
type TokenDiagnostic struct {
	Token      string          `json:"token"`
	Canonical  string          `json:"canonical,omitempty"`
	Variable   string          `json:"variable,omitempty"`
	Constraint ValueConstraint `json:"constraint,omitempty"`
	Status     TokenStatus     `json:"status"`
	Consumer   string          `json:"consumer,omitempty"`
	Reason     string          `json:"reason,omitempty"`
}

// CSSProjection contains safe variables, deterministic inline declarations,
// and projection/support diagnostics.
type CSSProjection struct {
	Variables   map[string]string `json:"variables"`
	Inline      string            `json:"inline"`
	Diagnostics []TokenDiagnostic `json:"diagnostics,omitempty"`
}

// TokenResolution reports which semantic token supplied a consumer value.
// An empty Token means the current package default was used.
type TokenResolution struct {
	Token string
	Value string
}

type projectionEntry struct {
	token         string
	canonical     string
	value         string
	variable      string
	constraint    ValueConstraint
	supported     bool
	aliased       bool
	canonicalWins bool
	valid         bool
	emit          bool
	reason        string
}

var (
	tokenNamePattern    = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)
	variableNamePattern = regexp.MustCompile(`^--[A-Za-z0-9_-]+$`)
	prefixPattern       = regexp.MustCompile(`^--[A-Za-z0-9_-]*$`)
	decimalPattern      = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)
	lengthPattern       = regexp.MustCompile(`^([+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+))(px|rem|em|%|vh|vw|ch)?$`)
	durationPattern     = regexp.MustCompile(`^([+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+))(ms|s)$`)
	hexColorPattern     = regexp.MustCompile(`^#[0-9A-Fa-f]{3}(?:[0-9A-Fa-f]{1}|[0-9A-Fa-f]{3}|[0-9A-Fa-f]{5})?$`)
	colorFunction       = regexp.MustCompile(`(?i)^(rgb|rgba|hsl|hsla)\((.*)\)$`)
	dangerousFunction   = regexp.MustCompile(`(?i)(?:url|expression)\s*\(`)
	dangerousScheme     = regexp.MustCompile(`(?i)(?:javascript|vbscript|data)\s*:`)
)

var portableTokenSpecs = map[string]TokenSpec{
	"chart.axis":                 {Constraint: ConstraintColor},
	"chart.grid":                 {Constraint: ConstraintColor},
	"chart.series.1":             {Constraint: ConstraintColor},
	"chart.series.2":             {Constraint: ConstraintColor},
	"chart.series.3":             {Constraint: ConstraintColor},
	"chart.series.4":             {Constraint: ConstraintColor},
	"chart.series.5":             {Constraint: ConstraintColor},
	"chart.series.6":             {Constraint: ConstraintColor},
	"chart.series.7":             {Constraint: ConstraintColor},
	"chart.series.8":             {Constraint: ConstraintColor},
	"chart.tooltip-surface":      {Constraint: ConstraintColor},
	"chart.tooltip-text":         {Constraint: ConstraintColor},
	"color.action.accent":        {Constraint: ConstraintColor},
	"color.action.primary":       {Constraint: ConstraintColor},
	"color.action.primary-hover": {Constraint: ConstraintColor},
	"color.border.default":       {Constraint: ConstraintColor},
	"color.border.strong":        {Constraint: ConstraintColor},
	"color.focus.ring":           {Constraint: ConstraintColor},
	"color.status.danger":        {Constraint: ConstraintColor},
	"color.status.info":          {Constraint: ConstraintColor},
	"color.status.success":       {Constraint: ConstraintColor},
	"color.status.warning":       {Constraint: ConstraintColor},
	"color.surface.canvas":       {Constraint: ConstraintColor},
	"color.surface.default":      {Constraint: ConstraintColor},
	"color.surface.raised":       {Constraint: ConstraintColor},
	"color.surface.subtle":       {Constraint: ConstraintColor},
	"color.text.inverse":         {Constraint: ConstraintColor},
	"color.text.primary":         {Constraint: ConstraintColor},
	"color.text.secondary":       {Constraint: ConstraintColor},
	"font.family.body":           {Constraint: ConstraintFontFamily},
	"font.family.heading":        {Constraint: ConstraintFontFamily},
	"font.size.body":             {Constraint: ConstraintNonnegativeLength},
	"font.size.label":            {Constraint: ConstraintNonnegativeLength},
	"font.weight.body":           {Constraint: ConstraintFontWeight},
	"font.weight.emphasis":       {Constraint: ConstraintFontWeight},
	"letter.spacing.body":        {Constraint: ConstraintLength},
	"line.height.body":           {Constraint: ConstraintNumber},
	"motion.duration.fast":       {Constraint: ConstraintDuration},
	"motion.duration.normal":     {Constraint: ConstraintDuration},
	"motion.easing.standard":     {Constraint: ConstraintEasing},
	"radius.control":             {Constraint: ConstraintNonnegativeLength},
	"radius.surface":             {Constraint: ConstraintNonnegativeLength},
	"shadow.surface":             {Constraint: ConstraintShadow},
	"size.control.height":        {Constraint: ConstraintNonnegativeLength},
	"space.control.block":        {Constraint: ConstraintNonnegativeLength},
	"space.control.inline":       {Constraint: ConstraintNonnegativeLength},
	"space.stack":                {Constraint: ConstraintNonnegativeLength},
	"space.surface":              {Constraint: ConstraintNonnegativeLength},
}

var dashboardTokenSpecs = map[string]TokenSpec{
	"dashboard.surface":               {Constraint: ConstraintColor},
	"dashboard.card.background":       {Constraint: ConstraintColor},
	"dashboard.card.border":           {Constraint: ConstraintColor},
	"dashboard.card.radius":           {Constraint: ConstraintNonnegativeLength},
	"dashboard.card.shadow":           {Constraint: ConstraintShadow},
	"dashboard.metric.label":          {Constraint: ConstraintColor},
	"dashboard.metric.value":          {Constraint: ConstraintColor},
	"dashboard.metric.trend-positive": {Constraint: ConstraintColor},
	"dashboard.metric.trend-negative": {Constraint: ConstraintColor},
	"dashboard.chart.axis":            {Constraint: ConstraintColor},
	"dashboard.chart.grid":            {Constraint: ConstraintColor},
	"dashboard.chart.tooltip-surface": {Constraint: ConstraintColor},
	"dashboard.chart.tooltip-text":    {Constraint: ConstraintColor},
}

var portableAliases = map[string]string{
	"accent":  "color.action.accent",
	"primary": "color.action.primary",
	"surface": "color.surface.default",
}

// PortableSemanticProfile returns the renderer-independent fixture profile.
func PortableSemanticProfile() TokenProfile {
	return TokenProfile{
		Name:    "portable",
		Tokens:  cloneTokenSpecs(portableTokenSpecs),
		Aliases: cloneStringValues(portableAliases),
	}
}

// DashboardSemanticProfile returns the portable profile plus dashboard-owned
// extensions. Returned maps are defensive copies.
func DashboardSemanticProfile() TokenProfile {
	profile := PortableSemanticProfile()
	profile.Name = "go-dashboard"
	maps.Copy(profile.Tokens, dashboardTokenSpecs)
	return profile
}

// SemanticProjection safely projects a selection using the dashboard profile.
func (theme *ThemeSelection) SemanticProjection() CSSProjection {
	if theme == nil {
		return CSSProjection{Variables: map[string]string{}}
	}
	profile := DashboardSemanticProfile()
	return ProjectThemeCSSVariables(theme.Tokens, ProjectionOptions{Profile: &profile})
}

// SemanticDashboardEnabled reports whether a canonical portable or
// dashboard-owned token is present and valid. Legacy payloads do not opt into
// the semantic consumer stylesheet merely because they contain safe tokens.
func (theme *ThemeSelection) SemanticDashboardEnabled() bool {
	if theme == nil {
		return false
	}
	valid := map[string]bool{}
	for _, diagnostic := range theme.SemanticProjection().Diagnostics {
		if diagnostic.Status == TokenResolved && diagnostic.Token == diagnostic.Canonical {
			valid[diagnostic.Canonical] = true
		}
	}
	for _, diagnostic := range theme.SemanticProjection().Diagnostics {
		if diagnostic.Status == TokenSupported &&
			valid[diagnostic.Canonical] &&
			dashboardChromeTokens[diagnostic.Canonical] {
			return true
		}
	}
	return false
}

// DashboardConsumerDiagnostics reports the tokens actually selected by the
// generic dashboard shell, widget chrome, metrics, and state consumers.
func (theme *ThemeSelection) DashboardConsumerDiagnostics() []TokenDiagnostic {
	projection := theme.SemanticProjection()
	if theme == nil || !theme.SemanticDashboardEnabled() {
		return projection.Diagnostics
	}
	consumed := make([]string, 0, len(dashboardChromeFallbacks))
	seen := map[string]bool{}
	for _, fallback := range dashboardChromeFallbacks {
		resolved := theme.ResolveSemanticToken(fallback.component, fallback.portable, "")
		if resolved.Token != "" && !seen[resolved.Token] {
			seen[resolved.Token] = true
			consumed = append(consumed, resolved.Token)
		}
	}
	return projection.ConsumerDiagnostics("go-dashboard.template", consumed...)
}

var dashboardChromeFallbacks = []struct {
	component string
	portable  []string
}{
	{"dashboard.surface", []string{"color.surface.canvas"}},
	{"dashboard.card.background", []string{"color.surface.raised", "color.surface.default"}},
	{"dashboard.card.border", []string{"color.border.default"}},
	{"dashboard.card.radius", []string{"radius.surface"}},
	{"dashboard.card.shadow", []string{"shadow.surface"}},
	{"dashboard.metric.label", []string{"color.text.secondary"}},
	{"dashboard.metric.value", []string{"color.text.primary"}},
	{"dashboard.metric.trend-positive", []string{"color.status.success"}},
	{"dashboard.metric.trend-negative", []string{"color.status.danger"}},
	{"", []string{"color.focus.ring"}},
	{"", []string{"font.family.body"}},
	{"", []string{"font.family.heading", "font.family.body"}},
	{"", []string{"motion.duration.fast"}},
	{"", []string{"motion.easing.standard"}},
	{"", []string{"space.stack"}},
}

var dashboardChromeTokens = func() map[string]bool {
	tokens := map[string]bool{}
	for _, fallback := range dashboardChromeFallbacks {
		if fallback.component != "" {
			tokens[fallback.component] = true
		}
		for _, token := range fallback.portable {
			tokens[token] = true
		}
	}
	return tokens
}()

// ResolveSemanticToken applies the frozen component -> portable -> current
// default fallback chain and ignores values that fail their profile constraint.
func (theme *ThemeSelection) ResolveSemanticToken(component string, portable []string, current string) TokenResolution {
	if theme == nil {
		return TokenResolution{Value: current}
	}
	profile := DashboardSemanticProfile()
	candidates := make([]string, 0, 1+len(portable))
	if component != "" {
		candidates = append(candidates, component)
	}
	candidates = append(candidates, portable...)
	for _, token := range candidates {
		value, ok := theme.Tokens[token]
		if !ok {
			continue
		}
		spec, supported := profile.Tokens[token]
		if !supported {
			continue
		}
		value = strings.TrimSpace(value)
		if validateTokenValue(token, value, normalizeConstraint(spec.Constraint)) {
			return TokenResolution{Token: token, Value: value}
		}
	}
	return TokenResolution{Value: current}
}

// ConsumerDiagnostics appends deterministic consumed/unused outcomes for valid
// supported tokens while retaining projection and support diagnostics.
func (projection CSSProjection) ConsumerDiagnostics(consumer string, consumed ...string) []TokenDiagnostic {
	out := append([]TokenDiagnostic(nil), projection.Diagnostics...)
	consumedSet := make(map[string]bool, len(consumed))
	for _, token := range consumed {
		consumedSet[token] = true
	}
	valid := make(map[string]bool, len(projection.Diagnostics))
	for _, diagnostic := range projection.Diagnostics {
		if diagnostic.Status == TokenResolved {
			valid[diagnostic.Token] = true
		}
	}
	for _, diagnostic := range projection.Diagnostics {
		if diagnostic.Status != TokenSupported || !valid[diagnostic.Token] {
			continue
		}
		status := TokenUnused
		if consumedSet[diagnostic.Canonical] || consumedSet[diagnostic.Token] {
			status = TokenConsumed
		}
		diagnostic.Status = status
		diagnostic.Consumer = consumer
		diagnostic.Reason = ""
		out = append(out, diagnostic)
	}
	return out
}

// ProjectThemeCSSVariables mirrors the coordinated go-theme projection
// contract without creating a runtime dependency on go-theme.
func ProjectThemeCSSVariables(tokens map[string]string, options ProjectionOptions) CSSProjection {
	out := CSSProjection{Variables: map[string]string{}}
	if len(tokens) == 0 {
		return out
	}

	prefix := options.Prefix
	if prefix == "" {
		prefix = "--"
	}
	prefixValid := prefixPattern.MatchString(prefix)
	keys := sortedTokenKeys(tokens)

	canonicalPresent := make(map[string]bool, len(tokens))
	for _, token := range keys {
		canonical, _ := canonicalToken(token, options.Profile)
		if canonical == token {
			canonicalPresent[canonical] = true
		}
	}

	entries := make([]projectionEntry, 0, len(keys))
	for _, token := range keys {
		value := strings.TrimSpace(tokens[token])
		canonical, aliased := canonicalToken(token, options.Profile)
		spec, supported := tokenSpec(canonical, options.Profile)
		constraint := normalizeConstraint(spec.Constraint)
		variable, nameValid := projectedVariable(canonical, spec.Variable, prefix, prefixValid)
		tokenNameValid := tokenNamePattern.MatchString(token)
		canonicalNameValid := tokenNamePattern.MatchString(canonical)
		valueValid := validateTokenValue(canonical, value, constraint)
		canonicalWins := aliased && canonicalPresent[canonical]

		reason := ""
		valid := prefixValid && tokenNameValid && canonicalNameValid && nameValid && valueValid
		switch {
		case !prefixValid:
			reason = "invalid CSS variable prefix"
		case !tokenNameValid:
			reason = "invalid token name"
		case !canonicalNameValid:
			reason = "invalid canonical token name"
		case !nameValid:
			reason = "invalid CSS variable name"
		case !valueValid:
			reason = "invalid " + string(constraint) + " value"
		case canonicalWins:
			reason = "deprecated alias ignored because canonical token is present"
		}

		entries = append(entries, projectionEntry{
			token:         token,
			canonical:     canonical,
			value:         value,
			variable:      variable,
			constraint:    constraint,
			supported:     supported,
			aliased:       aliased,
			canonicalWins: canonicalWins,
			valid:         valid,
			emit:          valid && !canonicalWins,
			reason:        reason,
		})
	}

	collisions := make(map[string][]int)
	for index := range entries {
		if entries[index].emit {
			collisions[entries[index].variable] = append(collisions[entries[index].variable], index)
		}
	}
	for _, indexes := range collisions {
		if len(indexes) < 2 {
			continue
		}
		collidingTokens := make([]string, 0, len(indexes))
		for _, index := range indexes {
			collidingTokens = append(collidingTokens, entries[index].token)
		}
		reason := "CSS variable collision with tokens: " + strings.Join(collidingTokens, ", ")
		for _, index := range indexes {
			entries[index].valid = false
			entries[index].emit = false
			entries[index].reason = reason
		}
	}

	var inline strings.Builder
	for _, entry := range entries {
		status := TokenResolved
		if !entry.valid {
			status = TokenInvalid
		}
		out.Diagnostics = append(out.Diagnostics, TokenDiagnostic{
			Token:      entry.token,
			Canonical:  entry.canonical,
			Variable:   entry.variable,
			Constraint: entry.constraint,
			Status:     status,
			Reason:     entry.reason,
		})
		if entry.emit {
			out.Variables[entry.variable] = entry.value
			inline.WriteString(entry.variable)
			inline.WriteByte(':')
			inline.WriteString(entry.value)
			inline.WriteByte(';')
		}
		if options.Profile != nil && prefixValid {
			out.Diagnostics = append(out.Diagnostics, supportDiagnostic(entry, options.Profile.Name))
		}
	}
	out.Inline = inline.String()
	return out
}

func cloneTokenSpecs(src map[string]TokenSpec) map[string]TokenSpec {
	dst := make(map[string]TokenSpec, len(src))
	maps.Copy(dst, src)
	return dst
}

func cloneStringValues(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	maps.Copy(dst, src)
	return dst
}

func sortedTokenKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func canonicalToken(token string, profile *TokenProfile) (string, bool) {
	if profile == nil {
		return token, false
	}
	canonical, ok := profile.Aliases[token]
	if !ok || strings.TrimSpace(canonical) == "" {
		return token, false
	}
	return canonical, true
}

func tokenSpec(canonical string, profile *TokenProfile) (TokenSpec, bool) {
	if profile == nil {
		return TokenSpec{Constraint: ConstraintCSSValue}, false
	}
	spec, ok := profile.Tokens[canonical]
	if !ok {
		return TokenSpec{Constraint: ConstraintCSSValue}, false
	}
	return spec, true
}

func normalizeConstraint(constraint ValueConstraint) ValueConstraint {
	if constraint == "" {
		return ConstraintCSSValue
	}
	return constraint
}

func projectedVariable(token, configured, prefix string, prefixValid bool) (string, bool) {
	if !prefixValid {
		return "", false
	}
	name := strings.TrimSpace(configured)
	if name == "" {
		name = strings.TrimPrefix(token, "--")
	} else if after, ok := strings.CutPrefix(name, "--"); ok {
		name = after
		if name == "" {
			return "", false
		}
	}
	if !tokenNamePattern.MatchString(name) {
		return "", false
	}
	name = strings.NewReplacer(".", "-", "_", "-").Replace(name)
	variable := prefix + name
	return variable, variableNamePattern.MatchString(variable)
}

func supportDiagnostic(entry projectionEntry, consumer string) TokenDiagnostic {
	status := TokenUnsupported
	reason := ""
	if entry.supported {
		status = TokenSupported
	}
	if entry.aliased {
		reason = "deprecated alias for " + entry.canonical
		if entry.canonicalWins {
			reason += "; canonical token takes precedence"
		}
	} else if !entry.supported {
		name := strings.TrimSpace(consumer)
		if name == "" {
			name = "profile"
		}
		reason = "token is not declared by profile " + name
	}
	return TokenDiagnostic{
		Token:      entry.token,
		Canonical:  entry.canonical,
		Variable:   entry.variable,
		Constraint: entry.constraint,
		Status:     status,
		Consumer:   consumer,
		Reason:     reason,
	}
}

func validateTokenValue(token, value string, constraint ValueConstraint) bool {
	switch constraint {
	case ConstraintCSSValue:
		return validCSSValue(value)
	case ConstraintColor:
		return validColor(value)
	case ConstraintLength:
		_, ok := parseLength(value, false)
		return ok
	case ConstraintNonnegativeLength:
		_, ok := parseLength(value, true)
		return ok
	case ConstraintNumber:
		number, ok := parseDecimal(value)
		if token == "line.height.body" {
			return ok && number > 0
		}
		return ok
	case ConstraintDuration:
		return validDuration(value)
	case ConstraintEasing:
		return validEasing(value)
	case ConstraintFontFamily:
		return validFontFamily(value)
	case ConstraintFontWeight:
		return validFontWeight(value)
	case ConstraintShadow:
		return validShadow(value)
	default:
		return false
	}
}

func validCSSValue(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !unsafeCSSSyntax(value, false)
}

func unsafeCSSSyntax(value string, allowQuotes bool) bool {
	for _, r := range value {
		if unicode.IsControl(r) || r == ';' || r == '{' || r == '}' || r == '<' ||
			r == '>' || r == '`' || r == '\\' {
			return true
		}
		if !allowQuotes && (r == '\'' || r == '"') {
			return true
		}
	}
	return strings.Contains(value, "/*") ||
		strings.Contains(value, "*/") ||
		dangerousFunction.MatchString(value) ||
		strings.Contains(strings.ToLower(value), "@import") ||
		dangerousScheme.MatchString(value)
}

func validColor(value string) bool {
	value = strings.TrimSpace(value)
	if value == "transparent" || value == "currentColor" || hexColorPattern.MatchString(value) {
		return true
	}
	if unsafeCSSSyntax(value, false) {
		return false
	}
	matches := colorFunction.FindStringSubmatch(value)
	if len(matches) != 3 {
		return false
	}
	parts := splitAndTrim(matches[2], ",")
	switch strings.ToLower(matches[1]) {
	case "rgb":
		return len(parts) == 3 && validRGBChannels(parts)
	case "rgba":
		return len(parts) == 4 && validRGBChannels(parts[:3]) && validAlpha(parts[3])
	case "hsl":
		return len(parts) == 3 && validHue(parts[0]) && validPercentage(parts[1]) && validPercentage(parts[2])
	case "hsla":
		return len(parts) == 4 && validHue(parts[0]) && validPercentage(parts[1]) &&
			validPercentage(parts[2]) && validAlpha(parts[3])
	default:
		return false
	}
}

func splitAndTrim(value, separator string) []string {
	raw := strings.Split(value, separator)
	out := make([]string, len(raw))
	for index, item := range raw {
		out[index] = strings.TrimSpace(item)
	}
	return out
}

func validRGBChannels(parts []string) bool {
	percentage := false
	for index, part := range parts {
		isPercentage := strings.HasSuffix(part, "%")
		if index == 0 {
			percentage = isPercentage
		} else if percentage != isPercentage {
			return false
		}
		number, ok := parseDecimal(strings.TrimSuffix(part, "%"))
		if !ok || number < 0 || percentage && number > 100 || !percentage && number > 255 {
			return false
		}
	}
	return true
}

func validHue(value string) bool {
	_, ok := parseDecimal(value)
	return ok
}

func validPercentage(value string) bool {
	if !strings.HasSuffix(value, "%") {
		return false
	}
	number, ok := parseDecimal(strings.TrimSuffix(value, "%"))
	return ok && number >= 0 && number <= 100
}

func validAlpha(value string) bool {
	if strings.HasSuffix(value, "%") {
		return validPercentage(value)
	}
	number, ok := parseDecimal(value)
	return ok && number >= 0 && number <= 1
}

func parseLength(value string, nonnegative bool) (float64, bool) {
	matches := lengthPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return 0, false
	}
	number, ok := parseDecimal(matches[1])
	if !ok || nonnegative && number < 0 || matches[2] == "" && number != 0 {
		return 0, false
	}
	return number, true
}

func parseDecimal(value string) (float64, bool) {
	if !decimalPattern.MatchString(value) {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
		return 0, false
	}
	return number, true
}

func validDuration(value string) bool {
	matches := durationPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return false
	}
	number, ok := parseDecimal(matches[1])
	return ok && number >= 0
}

func validEasing(value string) bool {
	value = strings.TrimSpace(value)
	switch value {
	case "linear", "ease", "ease-in", "ease-out", "ease-in-out":
		return true
	}
	if !strings.HasPrefix(value, "cubic-bezier(") || !strings.HasSuffix(value, ")") {
		return false
	}
	parts := splitAndTrim(strings.TrimSuffix(strings.TrimPrefix(value, "cubic-bezier("), ")"), ",")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if _, ok := parseDecimal(part); !ok {
			return false
		}
	}
	return true
}

func validFontFamily(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || unsafeCSSSyntax(value, true) || strings.Contains(value, `\`) {
		return false
	}
	var quote rune
	var familyHasContent bool
	for _, r := range value {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else if !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_') {
				return false
			} else if !unicode.IsSpace(r) {
				familyHasContent = true
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			if !familyHasContent {
				return false
			}
			familyHasContent = false
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_':
			if !unicode.IsSpace(r) {
				familyHasContent = true
			}
		default:
			return false
		}
	}
	return quote == 0 && familyHasContent
}

func validFontWeight(value string) bool {
	switch value {
	case "normal", "bold":
		return true
	}
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	weight, err := strconv.Atoi(value)
	return err == nil && weight >= 1 && weight <= 1000
}

func validShadow(value string) bool {
	value = strings.TrimSpace(value)
	if value == "none" {
		return true
	}
	if value == "" || unsafeCSSSyntax(value, false) {
		return false
	}
	layers, ok := splitShadowLayers(value)
	if !ok || len(layers) == 0 {
		return false
	}
	for _, layer := range layers {
		if !validShadowLayer(layer) {
			return false
		}
	}
	return true
}

func splitShadowLayers(value string) ([]string, bool) {
	var layers []string
	start := 0
	depth := 0
	for index, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, false
			}
		case ',':
			if depth == 0 {
				layer := strings.TrimSpace(value[start:index])
				if layer == "" {
					return nil, false
				}
				layers = append(layers, layer)
				start = index + 1
			}
		}
	}
	if depth != 0 {
		return nil, false
	}
	layer := strings.TrimSpace(value[start:])
	if layer == "" {
		return nil, false
	}
	return append(layers, layer), true
}

func validShadowLayer(layer string) bool {
	parts := splitShadowParts(layer)
	lengths := 0
	colorSeen := false
	insetSeen := false
	for _, part := range parts {
		switch {
		case part == "inset":
			if insetSeen {
				return false
			}
			insetSeen = true
		case validColor(part):
			if colorSeen {
				return false
			}
			colorSeen = true
		default:
			if _, ok := parseLength(part, false); !ok {
				return false
			}
			lengths++
		}
	}
	return lengths >= 2 && lengths <= 4
}

func splitShadowParts(layer string) []string {
	var parts []string
	start := -1
	depth := 0
	for index, r := range layer {
		if start == -1 && !unicode.IsSpace(r) {
			start = index
		}
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		}
		if start >= 0 && depth == 0 && unicode.IsSpace(r) {
			parts = append(parts, strings.TrimSpace(layer[start:index]))
			start = -1
		}
	}
	if start >= 0 {
		parts = append(parts, strings.TrimSpace(layer[start:]))
	}
	return parts
}
