//nolint:all // legacy BPE parser adaptation
package tokencounter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
	extfs "github.com/quenbyako/ext/fs"
)

type TokenizerConfig struct {
	ChatTemplate string `json:"chat_template"`
}

type PreTokenizerPattern struct {
	Regex string `json:"Regex"`
}

type PreTokenizerObj struct {
	Type          string               `json:"type"`
	Pattern       *PreTokenizerPattern `json:"pattern,omitempty"`
	PreTokenizers []PreTokenizerObj    `json:"pretokenizers,omitempty"`
}

type TokenizerJSON struct {
	PreTokenizer *PreTokenizerObj `json:"pre_tokenizer,omitempty"`
	Model        struct {
		Vocab  map[string]int `json:"vocab"`
		Merges []string       `json:"merges"`
	} `json:"model"`
	AddedTokens []struct {
		Content string `json:"content"`
		Special bool   `json:"special"`
		Id      int    `json:"id"`
	} `json:"added_tokens"`
}

func findRegexPattern(pt *PreTokenizerObj) string {
	if pt == nil {
		return ""
	}
	if pt.Pattern != nil && pt.Pattern.Regex != "" {
		return pt.Pattern.Regex
	}
	for i := range pt.PreTokenizers {
		if r := findRegexPattern(&pt.PreTokenizers[i]); r != "" {
			return r
		}
	}
	return ""
}

type pair struct {
	first, second string
}

type BPE struct {
	vocab         map[string]int
	ranks         map[pair]int
	b2u           map[byte]rune
	pattern       *regexp2.Regexp
	specialTokens map[string]int
	chatTemplate  string
}

func bytesToUnicode() map[byte]rune {
	b2u := make(map[byte]rune)

	var bs []int
	for b := int('!'); b <= int('~'); b++ {
		bs = append(bs, b)
	}

	for b := int('¡'); b <= int('¬'); b++ {
		bs = append(bs, b)
	}

	for b := int('®'); b <= int('ÿ'); b++ {
		bs = append(bs, b)
	}

	cs := make([]int, 0, len(bs)+256)
	cs = append(cs, bs...)

	n := 0

	for b := range 256 {
		found := false

		for _, x := range bs {
			if x == b {
				found = true
				break
			}
		}

		if !found {
			bs = append(bs, b)
			cs = append(cs, 256+n)
			n++
		}
	}

	for i := range len(bs) {
		b2u[byte(bs[i])] = rune(cs[i])
	}

	return b2u
}

func NewBPE(fsys extfs.FS, configPath, vocabPath string, special []string) (*BPE, error) {
	var config TokenizerConfig
	cData, err := extfs.ReadFile(fsys, configPath)
	if err == nil {
		_ = json.Unmarshal(cData, &config)
	}

	vData, err := extfs.ReadFile(fsys, vocabPath)
	if err != nil {
		return nil, fmt.Errorf("read tokenizer.json: %w", err)
	}

	var tokJSON TokenizerJSON
	if err := json.Unmarshal(vData, &tokJSON); err != nil {
		return nil, fmt.Errorf("unmarshal tokenizer.json: %w", err)
	}

	ranks := make(map[pair]int)
	for i, m := range tokJSON.Model.Merges {
		parts := strings.Split(m, " ")
		if len(parts) == 2 {
			ranks[pair{parts[0], parts[1]}] = i
		}
	}

	b2u := bytesToUnicode()

	patternStr := findRegexPattern(tokJSON.PreTokenizer)
	if patternStr == "" {
		// Fallback to standard cl100k_base / o200k_base / Qwen / DeepSeek BPE regex
		patternStr = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`
	}

	pattern, err := regexp2.Compile(patternStr, 0)
	if err != nil {
		return nil, fmt.Errorf("compile split pattern: %w", err)
	}

	specMap := make(map[string]int)
	for _, token := range special {
		tokenNorm := strings.ReplaceAll(token, "\u2581", " ")
		if id, ok := tokJSON.Model.Vocab[token]; ok {
			specMap[token] = id
			specMap[tokenNorm] = id
		} else {
			specMap[token] = 999999
			specMap[tokenNorm] = 999999
		}
	}

	// Parse added_tokens directly from tokenizer.json dynamically
	for _, val := range tokJSON.AddedTokens {
		tokenNorm := strings.ReplaceAll(val.Content, "\u2581", " ")
		id := val.Id
		if id == 0 {
			if vocID, ok := tokJSON.Model.Vocab[val.Content]; ok {
				id = vocID
			} else {
				id = 999999
			}
		}
		specMap[val.Content] = id
		specMap[tokenNorm] = id
	}

	return &BPE{
		vocab:         tokJSON.Model.Vocab,
		ranks:         ranks,
		b2u:           b2u,
		pattern:       pattern,
		specialTokens: specMap,
		chatTemplate:  config.ChatTemplate,
	}, nil
}

func (b *BPE) ApplyTemplate(msgs []Message) string {
	var sb strings.Builder

	for _, m := range msgs {
		sb.WriteString("<|im_start|>\n")
		sb.WriteString(m.Role)
		sb.WriteString("\n")
		sb.WriteString(m.Content)
		sb.WriteString("\n<|im_end|>\n")
	}

	sb.WriteString("<|im_start|>\nassistant\n")

	return sb.String()
}

func (b *BPE) Encode(text string) []int {
	if len(b.specialTokens) == 0 {
		return b.encodeNormalTokens(text)
	}

	var parts []string
	for k := range b.specialTokens {
		parts = append(parts, regexp.QuoteMeta(k))
	}

	specialRegex := regexp.MustCompile(strings.Join(parts, "|"))

	matches := specialRegex.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return b.encodeNormalTokens(text)
	}

	var tokens []int
	lastIdx := 0

	for _, match := range matches {
		start, end := match[0], match[1]
		if start > lastIdx {
			tokens = append(tokens, b.encodeNormalTokens(text[lastIdx:start])...)
		}

		tokStr := text[start:end]
		if id, ok := b.specialTokens[tokStr]; ok {
			tokens = append(tokens, id)
		} else {
			tokens = append(tokens, 999999)
		}

		lastIdx = end
	}

	if lastIdx < len(text) {
		tokens = append(tokens, b.encodeNormalTokens(text[lastIdx:])...)
	}

	return tokens
}

func (b *BPE) encodeNormalTokens(chunk string) []int {
	m, err := b.pattern.FindStringMatch(chunk)
	if err != nil || m == nil {
		return nil
	}

	var tokens []int

	for m != nil {
		subChunk := m.String()
		var mapped []string

		bytes := []byte(subChunk)
		for _, bt := range bytes {
			mapped = append(mapped, string(b.b2u[bt]))
		}

		merged := b.bpeMerge(mapped)
		for _, token := range merged {
			if id, ok := b.vocab[token]; ok {
				tokens = append(tokens, id)
			}
		}

		m, _ = b.pattern.FindNextMatch(m)
	}

	return tokens
}

func (b *BPE) bpeMerge(word []string) []string {
	if len(word) < 2 {
		return word
	}

	for {
		var (
			minRank  int = 1e9
			bestPair pair
			found    bool
		)

		for i := range len(word) - 1 {
			p := pair{word[i], word[i+1]}
			if rank, ok := b.ranks[p]; ok {
				if rank < minRank {
					minRank = rank
					bestPair = p
					found = true
				}
			}
		}

		if !found {
			break
		}

		var newWord []string

		for i := 0; i < len(word); {
			if i < len(word)-1 && word[i] == bestPair.first && word[i+1] == bestPair.second {
				newWord = append(newWord, bestPair.first+bestPair.second)
				i += 2
			} else {
				newWord = append(newWord, word[i])
				i++
			}
		}

		word = newWord
		if len(word) < 2 {
			break
		}
	}

	return word
}
