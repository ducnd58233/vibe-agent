package repomap

import (
	"slices"
	"testing"
)

// names is what a test asserts on. The line numbers matter to the renderer, not
// to whether extraction found the right things.
func names(symbols []Symbol) []string {
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		out = append(out, symbol.Name)
	}
	return out
}

func equal(got, want []string) bool {
	return slices.Equal(got, want)
}

func TestExtractFindsDeclarations(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
		want []string
	}{
		{
			name: "go",
			path: "internal/memory/sqlite.go",
			body: `package memory

import "database/sql"

type Store struct {
	db *sql.DB
}

const StateDirName = ".agent-state"

func Open(workspaceRoot string) (*Store, error) { return nil, nil }

func (s *Store) Propose(candidate Record) error { return nil }

// unexportedHelper is skipped: a map is for orienting in someone else's code,
// and package-private names do not orient anyone outside the package.
func unexportedHelper() {}
`,
			want: []string{"Store", "StateDirName", "Open", "Store.Propose"},
		},
		{
			name: "python",
			path: "api/routes.py",
			body: `import fastapi

DEFAULT_TIMEOUT = 30

class OrderService:
    def process(self, order):
        return order

    def _internal(self):
        pass

def create_app():
    return fastapi.FastAPI()

async def fetch_orders(session):
    return []
`,
			want: []string{"DEFAULT_TIMEOUT", "OrderService", "OrderService.process", "create_app", "fetch_orders"},
		},
		{
			name: "typescript",
			path: "src/orders.ts",
			body: `import { z } from "zod";

export const MAX_ITEMS = 50;

export interface Order {
  id: string;
}

export type OrderStatus = "open" | "closed";

export class OrderStore {
  find(id: string) {}
}

export function processOrder(order: Order) {}

export default async function handler() {}

function notExported() {}
`,
			want: []string{"MAX_ITEMS", "Order", "OrderStatus", "OrderStore", "processOrder", "handler"},
		},
		{
			name: "rust",
			path: "src/lib.rs",
			body: `use std::io;

pub const LIMIT: usize = 10;

pub struct Engine {
    size: usize,
}

pub trait Runner {
    fn run(&self);
}

pub fn build() -> Engine { Engine { size: 0 } }

fn private_helper() {}
`,
			want: []string{"LIMIT", "Engine", "Runner", "build"},
		},
		{
			name: "an unsupported language yields no symbols rather than noise",
			path: "config/nginx.conf",
			body: "server {\n  listen 80;\n}\n",
			want: nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := names(Extract(testCase.path, []byte(testCase.body)))
			if !equal(got, testCase.want) {
				t.Errorf("Extract(%s)\n got: %v\nwant: %v", testCase.path, got, testCase.want)
			}
		})
	}
}

// A map that reports a line number the reader cannot jump to is worse than one
// that reports none.
func TestExtractRecordsTheDeclarationLine(t *testing.T) {
	body := "package main\n\nfunc First() {}\n\nfunc Second() {}\n"
	symbols := Extract("main.go", []byte(body))
	if len(symbols) != 2 {
		t.Fatalf("want 2 symbols, got %d", len(symbols))
	}
	if symbols[0].Line != 3 {
		t.Errorf("First is on line %d, want 3", symbols[0].Line)
	}
	if symbols[1].Line != 5 {
		t.Errorf("Second is on line %d, want 5", symbols[1].Line)
	}
}

// A commented-out declaration is not a declaration. Counting one would put a
// symbol in the map that no reader can find in the code.
func TestExtractSkipsCommentedDeclarations(t *testing.T) {
	body := "package main\n\n// func Removed() {}\nfunc Kept() {}\n"
	if got := names(Extract("main.go", []byte(body))); !equal(got, []string{"Kept"}) {
		t.Errorf("got %v, want [Kept]", got)
	}
}

// Binary content reaches this only when something is misdetected upstream, and
// running regexes over a few megabytes of it costs real time for no symbols.
func TestExtractIgnoresBinaryContent(t *testing.T) {
	if got := Extract("assets/icon.go", []byte{0x00, 0x01, 0x02, 0xff, 'f', 'u', 'n', 'c'}); got != nil {
		t.Errorf("binary content produced symbols: %v", got)
	}
}
