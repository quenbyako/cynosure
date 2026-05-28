#!/usr/bin/env nu

# Nushell wrapper for golangci-lint
# Provides structured objects, statistics, and token-optimized compact outputs.

# Default command showing usage instructions
def main [] {
    print "Nushell wrapper for golangci-lint. Available subcommands:"
    print "  ./scripts/golangci.nu stats  # Get token-saving statistical summary of errors"
    print "  ./scripts/golangci.nu list   # List issues in a compact, token-saving format"
}

# Collect statistics on current linter issues (token-saving compact output)
def "main stats" [
    --dir (-d): path # Run only on specific directory/package path
] {
    let mapped_issues = (run-linter $dir)
    if ($mapped_issues | is-empty) {
        print "✅ No issues found"
        return
    }

    let total_count = ($mapped_issues | length)
    print $"📊 Total Linter Issues: ($total_count)\n"
    
    let by_category = ($mapped_issues | group-by category | transpose category issues | each { {category: $in.category, count: ($in.issues | length)} } | sort-by -r count)
    print "📁 Issues by Category:"
    $by_category | each { |c| print $"  - ($c.category): ($c.count)" } | ignore
    
    print "\n📦 Top Packages with Issues:"
    let by_package = ($mapped_issues | group-by pkg | transpose pkg issues | each { {pkg: $in.pkg, count: ($in.issues | length)} } | sort-by -r count | first 15)
    $by_package | each { |p| print $"  - ($p.pkg): ($p.count)" } | ignore
    
    print "\n🔍 Top Linters:"
    let by_linter = ($mapped_issues | group-by linter | transpose linter issues | each { {linter: $in.linter, count: ($in.issues | length)} } | sort-by -r count | first 10)
    $by_linter | each { |l| print $"  - ($l.linter): ($l.count)" } | ignore
}

# List issues in a compact, token-saving format grouped by package
def "main list" [
    --dir (-d): path       # Run only on specific directory/package path
    --limit (-l): int = 50 # Maximum number of issues to display
] {
    let mapped_issues = (run-linter $dir)
    if ($mapped_issues | is-empty) {
        print "✅ No issues found"
        return
    }

    let limited_issues = ($mapped_issues | first $limit)
    let grouped = ($limited_issues | group-by pkg)
    $grouped | transpose pkg issues | each { |row|
        print $"📦 ($row.pkg)"
        $row.issues | each { |issue|
            print $"  📄 ($issue.file):($issue.line):($issue.col) [($issue.linter)]"
            print $"     err : ($issue.message)"
        } | ignore
    } | ignore
    let total_count = ($mapped_issues | length)
    if $total_count > $limit {
        print $"\n⚠️ Shown ($limit) out of ($total_count) issues. Increase limit using --limit flag if needed."
    }
}

# Internal helper to run golangci-lint and parse issues
def run-linter [dir?: any] : nothing -> table<pkg: string, file: string, line: int, col: int, linter: string, category: string, message: string> {
    let target_path = (if ($dir | is-empty) { "./..." } else { $dir })
    let result = (do -i { golangci-lint run --output.json.path stdout $target_path } | complete)

    if $result.exit_code == 0 and ($result.stdout | is-empty) {
        return []
    }

    let lines = ($result.stdout | lines | str trim)
    let json_line = ($lines | where ($it | str starts-with "{") | default ["{}"] | first)
    let parsed = ($json_line | from json)
    let issues = ($parsed | get -o Issues | default [])

    if ($issues | is-empty) {
        return []
    }

    $issues | each { |issue|
        let filepath = $issue.Pos.Filename
        let pkg = ($filepath | path dirname)
        let file = ($filepath | path basename)
        let category = (categorize-linter $issue.FromLinter)
        {
            pkg: $pkg,
            file: $file,
            line: $issue.Pos.Line,
            col: $issue.Pos.Column,
            linter: $issue.FromLinter,
            category: $category,
            message: $issue.Text,
        }
    }
}

def categorize-linter [linter: string] {
    let easy = [
        gci, gofumpt, golines, whitespace, wsl_v5, tagalign, nlreturn,
        misspell, tagliatelle, goconst, copyloopvar, intrange, mirror,
        perfsprint, usestdlibvars
    ]
    let arch = [
        funlen, gocognit, maintidx, nestif, cyclop
    ]

    if $linter in $easy {
        "easy"
    } else if $linter in $arch {
        "architecture"
    } else {
        "complex"
    }
}
