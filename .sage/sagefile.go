package main

import (
	"context"

	"go.einride.tech/sage/sg"
	"go.einride.tech/sage/sgtool"
	"go.einride.tech/sage/tools/sgconvco"
	"go.einride.tech/sage/tools/sggit"
	"go.einride.tech/sage/tools/sggo"
	"go.einride.tech/sage/tools/sggolangcilintv2"
	"go.einride.tech/sage/tools/sgmdformat"
	"go.einride.tech/sage/tools/sgyamlfmt"
)

func main() {
	sg.GenerateMakefiles(
		sg.Makefile{
			Path:          sg.FromGitRoot("Makefile"),
			DefaultTarget: All,
		},
	)
}

func All(ctx context.Context) error {
	sg.Deps(ctx, ConvcoCheck, GoGenerate, GoLint, GoTest, FormatMarkdown, FormatYAML)
	sg.SerialDeps(ctx, GoModTidy, GitVerifyNoDiff)
	return nil
}

func FormatYAML(ctx context.Context) error {
	sg.Logger(ctx).Println("formatting YAML files...")
	return sgyamlfmt.Run(ctx)
}

func GoModTidy(ctx context.Context) error {
	sg.Logger(ctx).Println("tidying Go module files...")
	return sg.Command(ctx, "go", "mod", "tidy", "-v").Run()
}

func GoTest(ctx context.Context) error {
	sg.Logger(ctx).Println("running Go tests...")
	return sggo.TestCommand(ctx).Run()
}

// GoLint runs golangci-lint v2.
//
// sggolangcilint pins golangci-lint 1.64.8, which is built with Go 1.24 and
// refuses to analyze a module targeting anything newer. sage 0.416.1 requires
// Go 1.25, and sage lints every module in the repo including .sage itself, so
// the v1 tool cannot lint this repo at all once sage is on 1.25. golangci-lint
// v1 is end-of-life and will not gain Go 1.25 support, which is why sage ships
// sggolangcilintv2.
func GoLint(ctx context.Context) error {
	sg.Logger(ctx).Println("linting Go files...")
	return sggolangcilintv2.Run(ctx, sggolangcilintv2.Config{
		RunRelativePathMode: sggolangcilintv2.RunRelativePathModeGomod,
	})
}

func FormatMarkdown(ctx context.Context) error {
	sg.Logger(ctx).Println("formatting Markdown files...")
	return sgmdformat.Command(ctx).Run()
}

func ConvcoCheck(ctx context.Context) error {
	sg.Logger(ctx).Println("checking git commits...")
	return sgconvco.Command(ctx, "check", "origin/master..HEAD").Run()
}

func GitVerifyNoDiff(ctx context.Context) error {
	sg.Logger(ctx).Println("verifying that git has no diff...")
	return sggit.VerifyNoDiff(ctx)
}

func Stringer(ctx context.Context) error {
	sg.Logger(ctx).Println("building...")
	// x/tools v0.25.0 does not compile under Go 1.25: internal/tokeninternal
	// declares an array whose length the newer compiler evaluates as negative.
	_, err := sgtool.GoInstall(ctx, "golang.org/x/tools/cmd/stringer", "v0.39.0")
	return err
}

func GoGenerate(ctx context.Context) error {
	sg.Deps(ctx, Stringer)
	sg.Logger(ctx).Println("generating Go code...")
	return sg.Command(ctx, "go", "generate", "./...").Run()
}
