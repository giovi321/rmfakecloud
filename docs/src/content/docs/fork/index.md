---
title: This fork
---

`giovi321/rmfakecloud` exists to export reMarkable v6 notebooks and annotated PDFs
correctly. Upstream renders v6 notebooks as empty pages and places annotation ink by
fitting the device canvas onto the page, so a stock binary is a downgrade here rather
than a neutral swap.

Everything the fork changes has been measured against the tablet's own export rather
than reasoned about from the file format. Where a commit states a number, that number
came from a rendered comparison.

## What it fixes

**v6 export.** The `.rm` format version is decided per page instead of per document,
schema 2 page lists are read, `formatVersion 2` page tags decode, and pages render
individually with annotations laid over the source document. Page templates are read
from the device set, drawn under notebook ink, and can be added or removed from the
admin page.

**Ink placement on landscape pages.** A landscape page does not fit a portrait screen
the right way up, so the device turns the page and the ink is written turned with it.
The renderer drew that ink straight, which put annotations sideways on every landscape
document. Worked out by plotting raw stroke coordinates into the screen's own 1404x1872
frame and checking against the tablet's export of the same page. Turning the ink gives
one uniform scale: 0.5143 points per pixel across and 0.5146 down on a 960x540 page.

**Which page the ink lands on.** Only pages carrying ink were added to the archive's
page list, and the exporter takes each page's background from its position in that
list. On a 28 page document annotated on 14, the export had 14 pages with backgrounds
1 to 14 in order, and the ink from page 8 was drawn over the background of page 3. The
ink was confirmed as page 8's against a copy exported by the tablet itself, 99.9% of
pixels matching.

**The last page of a notebook.** This one lives in
[`giovi321/rmc-go`](https://github.com/giovi321/rmc-go), the renderer this fork calls.
`ExportToMultipagePDFCairo` emitted every page but the last, leaving cairo's `Finish`
to emit it, and cairo drops a blank page at finish. A notebook whose last page carried
no ink came out one page short. Found on two notebooks that each declared one more page
than the PDF held, both with an empty final `.rm` page of 424 and 325 bytes.

## Where it is going

Every fix above is open upstream, and if they land this fork stops being necessary.
[Branches and upstream PRs](branches/) tracks which is which.
