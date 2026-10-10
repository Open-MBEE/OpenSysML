package migrate_test

import (
	"archive/zip"
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// moduleSnapshot is a MagicDraw snapshot of the SysML profile module: the
// tool's block stereotypes, each generalizing «Block», as the archive declares them.
const moduleSnapshot = `<?xml version="1.0" encoding="ASCII"?>
<xmi:XMI xmi:version="2.0" xmlns:xmi="http://www.omg.org/XMI" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:uml="http://www.nomagic.com/magicdraw/UML/2.5.1.1">
  <uml:Package xmi:id="_md_sysml_root" ID="_md_sysml_root" name="SysML Profile">
    <packagedElement xsi:type="uml:Profile" xmi:id="_md_sysml" ID="_md_sysml" name="SysML">
      <packagedElement xsi:type="uml:Package" xmi:id="_md_blocks" ID="_md_blocks" name="Blocks">
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_md_block" ID="_md_block" name="Block"/>
      </packagedElement>
      <packagedElement xsi:type="uml:Package" xmi:id="_md_extensions" ID="_md_extensions" name="Non-Normative Extensions">
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_md_subsystem" ID="_md_subsystem" name="Subsystem">
          <generalization xmi:id="_md_subsystem_gen" ID="_md_subsystem_gen">
            <general xsi:type="uml:Stereotype" href="#_md_block"/>
          </generalization>
        </packagedElement>
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_md_system" ID="_md_system" name="System">
          <generalization xmi:id="_md_system_gen" ID="_md_system_gen" general="_md_block"/>
        </packagedElement>
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_md_external" ID="_md_external" name="External">
          <generalization xmi:id="_md_external_gen" ID="_md_external_gen">
            <general xsi:type="uml:Stereotype" href="#_md_block"/>
          </generalization>
        </packagedElement>
        <packagedElement xsi:type="uml:Stereotype" xmi:id="_md_domain" ID="_md_domain" name="Domain">
          <generalization xmi:id="_md_domain_gen" ID="_md_domain_gen">
            <general xsi:type="uml:Stereotype" href="#_md_block"/>
          </generalization>
        </packagedElement>
      </packagedElement>
    </packagedElement>
  </uml:Package>
</xmi:XMI>`

const moduleHref = "local:/PROJECT-44?resource=com.nomagic.magicdraw.uml_umodel.shared_umodel#_md_subsystem"

// plainSnapshot declares the same stereotypes with no generalization at all.
var plainSnapshot = regexp.MustCompile(`<generalization[^>]*/>|(?s)<generalization.*?</generalization>`).ReplaceAllString(moduleSnapshot, "")

// omgSnapshot declares no Block of its own: every generalization points by
// href into the OMG profile, which the archive does not bundle.
var omgSnapshot = strings.NewReplacer(
	`<packagedElement xsi:type="uml:Stereotype" xmi:id="_md_block" ID="_md_block" name="Block"/>`, "",
	`href="#_md_block"`, `href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#Block"`,
	`general="_md_block"`, `general="http://www.omg.org/spec/SysML/20181001/SysML.xmi#Block"`,
).Replace(moduleSnapshot)

// snapshotArchive zips the tool block stereotypes fixture with a snapshot of
// the SysML module, the table's row type pointing into the module.
func snapshotArchive(t *testing.T, snapshot string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/tool_block_stereotypes.xmi")
	if err != nil {
		t.Fatal(err)
	}
	model := strings.Replace(string(data),
		`<rowElementType href="http://www.omg.org/spec/SysML/20181001/SysML.xmi#SysML.Subsystem"/>`,
		`<rowElementType href="`+moduleHref+`"/>`, 1)
	if model == string(data) {
		t.Fatal("the fixture's table no longer names «Subsystem» by the OMG href")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := [][2]string{
		{"com.nomagic.ci.metamodel.project", `<?xml version="1.0"?><project/>`},
		{"com.nomagic.magicdraw.uml_model.model", model},
		{"proxy.local__PROJECT$h44_resource_com$dnomagic$dmagicdraw$duml_umodel$dshared_umodel$dsnapshot", snapshot},
	}
	for _, entry := range entries {
		w, err := zw.Create(entry[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entry[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A class applying the tool's «Subsystem» is a part def whether the archive's
// module snapshot declares the generalization to «Block» or only the tool's
// documentation does; the snapshot makes the class mapped rather than approximated.
func TestModuleSnapshotBlockSpecializations(t *testing.T) {
	r, err := migrate.Migrate("tool_block_stereotypes.mdzip", snapshotArchive(t, moduleSnapshot))
	if err != nil {
		t.Fatalf("Migrate(.mdzip): %v", err)
	}
	checkGolden(t, "testdata/xmi/tool_block_stereotypes.mdzip.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/tool_block_stereotypes.mdzip.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "tool_block_stereotypes.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	out := string(r.Notation)
	for _, want := range []string{
		"part def 'Beam Column' {", "part def Microscope {", "part def Laboratory {", "part def Imaging {",
		"occurrence def Paperwork {", "end fed : 'Beam Column';",
		`stereotype = "Subsystem";`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("snapshot migration lacks %q", want)
		}
	}
	if strings.Contains(report.String(), "tool's documented profile") {
		t.Errorf("the generalization the snapshot declares was taken from the tool table:\n%s", report.String())
	}
	if !strings.Contains(report.String(), "«Subsystem» specializes «Block» in the tool's profile") {
		t.Errorf("the table filtered by the module's «Subsystem» is not filtered as blocks:\n%s", report.String())
	}
	plain := migrateFixtureFile(t, "tool_block_stereotypes")
	for _, want := range []string{"part def 'Beam Column' {", "part def Microscope {", "occurrence def Paperwork {", `stereotype = "Subsystem";`} {
		if !strings.Contains(string(plain.Notation), want) {
			t.Errorf("plain migration lacks %q", want)
		}
	}
	if strings.Contains(string(plain.Notation), "applied stereotype «Subsystem»") {
		t.Error("the tool's «Subsystem» is kept as a comment, not metadata")
	}
}

// A user stereotype specializing «Block» in the document classifies as a block,
// and its own name is the metadata the class carries.
func TestUserBlockSpecializationsClassify(t *testing.T) {
	r := migrateFixtureFile(t, "block_specialization_profile")
	out := string(r.Notation)
	for _, want := range []string{
		"part def Telescope {", "part 'objective' : Lens;", "port 'light in' : iLight;", "@Optics::Segment;",
		"part def Tripod {", "@Optics::Mount;", "port def iLight {", "constraint def 'Focus Rule' {",
		"requirement 'Sharp Image' {", "end carried : Telescope;", `'metadata' = ("Optics::Segment")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("migration lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "occurrence def") {
		t.Errorf("a class whose stereotype specializes a standard one fell through to an occurrence def:\n%s", out)
	}
}

// A snapshot that declares «Subsystem» with no generalization is believed over
// the tool's documentation: the class is no block, and neither are table rows typed by it.
func TestModuleSnapshotWithoutBlockIsNotABlock(t *testing.T) {
	if !strings.Contains(plainSnapshot, `name="Subsystem"`) || strings.Contains(plainSnapshot, "generalization") {
		t.Fatalf("plain snapshot is not the module without generalizations:\n%s", plainSnapshot)
	}
	r, err := migrate.Migrate("tool_block_stereotypes.mdzip", snapshotArchive(t, plainSnapshot))
	if err != nil {
		t.Fatalf("Migrate(.mdzip): %v", err)
	}
	out := string(r.Notation)
	for _, want := range []string{"occurrence def 'Beam Column' {", "occurrence def Microscope {", "occurrence def Laboratory {", "occurrence def Imaging {", `stereotype = "Subsystem";`} {
		if !strings.Contains(out, want) {
			t.Errorf("migration lacks %q:\n%s", want, out)
		}
	}
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report.String(), "tool's documented profile") || !strings.Contains(report.String(), "no v2 metaclass stands for the elements of «Subsystem»") {
		t.Errorf("the snapshot's «Subsystem» was read from the tool table:\n%s", report.String())
	}
}

// Under -strict the applied specialization is still named, as a line comment.
func TestStrictKeepsBlockSpecializationName(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/tool_block_stereotypes.xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.MigrateOptions("tool_block_stereotypes.xmi", data, migrate.Options{Strict: true})
	if err != nil {
		t.Fatalf("MigrateOptions(strict): %v", err)
	}
	out := string(r.Notation)
	for _, want := range []string{"part def 'Beam Column' {", "// applied stereotype «Subsystem»", "// applied stereotype «System»"} {
		if !strings.Contains(out, want) {
			t.Errorf("strict migration lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "MigrationMetadata") {
		t.Errorf("strict migration names the OpenSysML library:\n%s", out)
	}
}

// A snapshot generalizing «Subsystem» by href into the unbundled OMG profile
// still makes the class a part def and filters table rows typed by it as blocks.
func TestModuleSnapshotGeneralizesIntoTheOMGProfile(t *testing.T) {
	if strings.Contains(omgSnapshot, `"_md_block"`) || strings.Contains(omgSnapshot, "#_md_block") || !strings.Contains(omgSnapshot, "SysML.xmi#Block") {
		t.Fatalf("OMG snapshot still declares or names a local Block:\n%s", omgSnapshot)
	}
	r, err := migrate.Migrate("tool_block_stereotypes.mdzip", snapshotArchive(t, omgSnapshot))
	if err != nil {
		t.Fatalf("Migrate(.mdzip): %v", err)
	}
	out := string(r.Notation)
	for _, want := range []string{"part def 'Beam Column' {", "part def Microscope {", "part def Laboratory {", "part def Imaging {", "occurrence def Paperwork {", `stereotype = "Subsystem";`} {
		if !strings.Contains(out, want) {
			t.Errorf("migration lacks %q:\n%s", want, out)
		}
	}
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	if text := report.String(); strings.Contains(text, "tool's documented profile") || strings.Contains(text, "no v2 metaclass stands for") || !strings.Contains(text, "«Subsystem» specializes «Block» in the tool's profile") {
		t.Errorf("the OMG Block general was not followed for the table's rows:\n%s", text)
	}
}
