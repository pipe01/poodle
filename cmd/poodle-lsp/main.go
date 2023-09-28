package main

import (
	goerrors "errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/pipe01/poodle/internal/lexer"
	"github.com/pipe01/poodle/internal/workspace"
	"github.com/tliron/commonlog"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"

	_ "github.com/tliron/commonlog/simple"
)

const lsName = "poodle"

var version string = "0.0.1"
var handler protocol.Handler

var documents = map[string]string{}

type SituatedErr interface {
	Unwrap() error
	At() lexer.Location
}

func main() {
	// This increases logging verbosity (optional)
	commonlog.Configure(1, nil)

	protocol.SetTraceValue(protocol.TraceValueMessage)

	handler = protocol.Handler{
		Initialize:  initialize,
		Initialized: initialized,
		Shutdown:    shutdown,
		SetTrace:    setTrace,
		TextDocumentDidOpen: func(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
			documents[params.TextDocument.URI] = params.TextDocument.Text

			return handleDocument(context, params.TextDocument.URI)
		},
		TextDocumentDidChange: func(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
			content, ok := documents[params.TextDocument.URI]
			if !ok {
				return nil
			}

			for _, change := range params.ContentChanges {
				switch change := change.(type) {
				case protocol.TextDocumentContentChangeEventWhole:
					documents[params.TextDocument.URI] = change.Text

				case protocol.TextDocumentContentChangeEvent:
					startIndex, endIndex := change.Range.IndexesIn(content)
					documents[params.TextDocument.URI] = content[:startIndex] + change.Text + content[endIndex:]
				}
			}

			return handleDocument(context, params.TextDocument.URI)
		},
		// TextDocumentSemanticTokensFull: semanticTokensFull,
	}

	server := server.NewServer(&handler, lsName, false)

	server.RunStdio()
}

func handleDocument(context *glsp.Context, docURI string) error {
	url, err := url.Parse(docURI)
	if err != nil {
		return fmt.Errorf("parse document uri: %w", err)
	}
	if url.Scheme != "file" {
		return fmt.Errorf("invalid document uri scheme %q", url.Scheme)
	}

	contents, ok := documents[docURI]
	if !ok {
		return nil
	}

	filePath := url.Path
	fileName := filepath.Base(filePath)

	ws := workspace.New(filepath.Dir(url.Path))

	// protocol.Trace(context, protocol.MessageTypeInfo, docURI)

	diag := []protocol.Diagnostic{}

	_, err = ws.LoadWithContents(fileName, []byte(contents))
	if err != nil {
		var poserr SituatedErr

		if goerrors.As(err, &poserr) {
			diag = append(diag, protocol.Diagnostic{
				Range: protocol.Range{
					Start: pos(poserr.At()),
					End:   pos(poserr.At()),
				},
				Severity: ptr(protocol.DiagnosticSeverityError),
				Message:  poserr.Unwrap().Error(),
			})
		} else {
			diag = append(diag, protocol.Diagnostic{
				Severity: ptr(protocol.DiagnosticSeverityError),
				Message:  err.Error(),
			})
		}
	}

	context.Notify(protocol.ServerTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         docURI,
		Diagnostics: diag,
	})

	return nil
}

func initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	capabilities := handler.CreateServerCapabilities()
	// capabilities.SemanticTokensProvider = &protocol.SemanticTokensOptions{
	// 	Legend: protocol.SemanticTokensLegend{
	// 		TokenTypes: []string{
	// 			"keyword",
	// 			"string",
	// 		},
	// 	},
	// 	Range: false,
	// 	Full:  true,
	// }

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}, nil
}

func initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func shutdown(context *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func semanticTokensFull(context *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	content, ok := documents[params.TextDocument.URI]
	if !ok {
		return nil, fmt.Errorf("document %q not found", params.TextDocument.URI)
	}

	l := lexer.New([]byte(content), filepath.Base(params.TextDocument.URI))

	tokens := make([]protocol.UInteger, 0)

	var prevPos lexer.Location
	for {
		tk, err := l.Next()
		if err != nil {
			return nil, fmt.Errorf("get next token: %w", err)
		}
		if tk.Type == lexer.TokenEOF {
			break
		}

		var tokenType protocol.UInteger
		shouldSend := true

		switch tk.Type {
		case lexer.TokenKeyword, lexer.TokenIdentifier:
			tokenType = 0

		case lexer.TokenAttributeName, lexer.TokenClassName, lexer.TokenQuotedString:
			tokenType = 1

		default:
			shouldSend = false
		}

		if shouldSend {
			var startDelta protocol.UInteger
			if tk.Start.Line == prevPos.Line {
				startDelta = uint32(tk.Start.Column - prevPos.Column)
			} else {
				startDelta = uint32(tk.Start.Column)
			}

			tokens = append(tokens,
				protocol.UInteger(tk.Start.Line-prevPos.Line),
				startDelta,
				protocol.UInteger(len(tk.Contents)),
				tokenType,
				0,
			)

			prevPos = tk.Start
			prevPos.Column += len(tk.Contents)
		}
	}

	return &protocol.SemanticTokens{
		Data: tokens,
	}, nil
}

func ptr[T any](v T) *T {
	return &v
}

func pos(l lexer.Location) protocol.Position {
	return protocol.Position{
		Line:      uint32(l.Line),
		Character: uint32(l.Column),
	}
}
