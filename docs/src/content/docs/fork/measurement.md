---
title: How the placement was measured
---

The constants in the exporter were read off rendered output rather than derived from
the file format. The scripts that did the reading are on the `measurement-tools`
branch under `tools/measure/`: about forty Python scripts and two shell helpers.

They are not a test suite and nothing runs them automatically. Most expect exported
PDFs in a working directory and need `pypdf`; the rasterising ones also need a
renderer. Document identifiers in them are placeholders, so substitute a real one
before running.

What they do, in groups:

- page geometry: sizes, scale, offsets, and where the ink bounding box lands on the page
- fitting the transform between device pixels and page points
- rendering pages to pixels and comparing them
- assertions over exported PDFs: page counts, size runs in page order, which pages
  carry text

Two measurements are worth stating on their own, because they are what made the fixes
verifiable rather than plausible.

**The landscape scale.** Plotting raw stroke coordinates into the device's own
1404x1872 frame shows the handwriting reading sideways. Turning it gives one uniform
scale, the page width over the screen's long side: 0.5143 points per pixel across and
0.5146 down on a 960x540 page. One scale in both axes is the evidence that the turn is
right, because a wrong rotation leaves the two axes disagreeing.

**The page pairing.** On a 28 page document annotated on 14, the export held 14 pages,
their backgrounds were source pages 1 to 14 in order, and the ink from page 8 was drawn
over the background of page 3. That ink was confirmed as page 8's against a copy of the
same page exported by the tablet itself, with 99.9% of pixels matching.

A test-only helper on the `points-probe` branch,
`internal/encoding/rm/points_probe_test.go`, dumps a page's stroke points one per line
given `RMFAKECLOUD_RM_FILE` and `RMFAKECLOUD_POINTS_OUT`. It produced the coordinates
those plots were made from.
