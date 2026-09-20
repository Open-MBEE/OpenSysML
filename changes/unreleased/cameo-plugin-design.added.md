- **A design note for a Cameo Systems Modeler plugin** (`docs/internals/design/cameo-plugin.md`).
  It answers, from the vendor's public documentation and Javadoc, how a plugin for Cameo 2024x
  Refresh3 (bundled JDK 17) is declared, loaded and distributed; where it contributes browser and
  diagram actions, a docking results panel and progress with cancel; how a selection leaves the
  tool (a saved `.mdzip` today, since no dialog-free OMG XMI 2.5 export was found in the OpenAPI);
  how a Cameo `xmi:id` maps to the SysML v2 name the migration writes through the per-element
  migration report, and what the service must return for that; how verdicts land on elements
  through annotations or a custom table; what the Simulation Toolkit covers and what OpenSysML
  adds; and whether the release offers SysML v2 (none found for 2024x Refresh3; 2026x does). A benchmark
  converts twenty public SysML v1 models — the repository's XMI fixtures, Cameo `.mdzip` projects
  and Papyrus models — and tabulates mapped, approximated, unmapped and skipped elements. It closes
  with the proposed `editors/cameo/` layout and build, the *Run with OpenSysML* sequence, a phased
  plan and the risks, every claim that could not be verified marked as such. Nothing is
  implemented; `editors/README.md` now lists the entry.
