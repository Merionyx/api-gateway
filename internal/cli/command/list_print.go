package command

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
	"github.com/merionyx/api-gateway/internal/cli/resource"
	"github.com/merionyx/api-gateway/internal/cli/style"
)

func printListTable(w io.Writer, color bool, kind resource.Kind, v any) {
	switch kind {
	case resource.Controllers:
		x := v.(*apiserverclient.ControllerListResponse)
		printControllerTable(w, x.Items)
		writePaginationFooter(w, color, x.HasMore, x.NextCursor)
	case resource.Tenants:
		x := v.(*apiserverclient.TenantListResponse)
		printTenantTable(w, x.Items)
		writePaginationFooter(w, color, x.HasMore, x.NextCursor)
	case resource.Environments:
		x := v.(*apiserverclient.EnvironmentListResponse)
		printEnvironmentTable(w, x.Items)
		writePaginationFooter(w, color, x.HasMore, x.NextCursor)
	case resource.Bundles:
		x := v.(*apiserverclient.BundleRefListResponse)
		printTenantBundleTable(w, x.Items)
		writePaginationFooter(w, color, x.HasMore, x.NextCursor)
	case resource.BundleKeys:
		x := v.(*apiserverclient.BundleRefListResponse)
		printTenantBundleTable(w, x.Items)
		writePaginationFooter(w, color, x.HasMore, x.NextCursor)
	case resource.ContractNames:
		x := v.(*apiserverclient.ContractNameListResponse)
		printContractNameTable(w, x.Items)
		writePaginationFooter(w, color, x.HasMore, x.NextCursor)
	default:
		_, _ = fmt.Fprintf(w, "%v\n", v)
	}
}

func printControllerTable(w io.Writer, items []apiserverclient.Controller) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTROLLER_ID\tTENANT")
	for _, c := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\n", c.ControllerId, c.Tenant)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func printTenantTable(w io.Writer, items []string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TENANT")
	for _, t := range items {
		_, _ = fmt.Fprintln(tw, t)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func printEnvironmentTable(w io.Writer, items []apiserverclient.Environment) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tBUNDLES")
	for _, e := range items {
		n := 0
		if e.Bundles != nil {
			n = len(*e.Bundles)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%d\n", e.Name, n)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func printTenantBundleTable(w io.Writer, items []apiserverclient.Bundle) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tKEY\tREPOSITORY\tREF\tPATH")
	for _, b := range items {
		name, key, repo, ref, path := deref(b.Name), deref(b.Key), deref(b.Repository), deref(b.Ref), deref(b.Path)
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", name, key, repo, ref, path)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func printContractNameTable(w io.Writer, items []string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTRACT_NAME")
	for _, n := range items {
		_, _ = fmt.Fprintln(tw, n)
	}
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
}

func writePaginationFooter(w io.Writer, color bool, hasMore bool, next *string) {
	if !hasMore && (next == nil || strings.TrimSpace(*next) == "") {
		return
	}
	var b strings.Builder
	_, _ = b.WriteString("has_more: ")
	if hasMore {
		_, _ = b.WriteString("true")
	} else {
		_, _ = b.WriteString("false")
	}
	if next != nil && strings.TrimSpace(*next) != "" {
		_, _ = fmt.Fprintf(&b, "  next_cursor: %s", *next)
	}
	_, _ = fmt.Fprintln(w, style.S(color, style.Dim, b.String()))
}
