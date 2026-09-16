# Measurement tools

Ad-hoc scripts used to work out how a v6 page's ink maps onto the document it
annotates. The constants in the exporter came from these measurements rather
than from reading the format, which is why they are kept.

They are not a test suite and nothing runs them automatically. Most expect
exported PDFs in `/check` or `/work/pdfs` and need `pypdf`; the rasterising ones
also need a renderer on the host.

**Document identifiers are redacted.** Substitute a real rmfakecloud document id
wherever a `DOCUMENT-ID` placeholder appears.

Roughly what is here:

- `geom.py`, `geo2.py`, `lezgeo.py`, `gengeom.py`, `shapes.py`, `sizes.py`,
  `side.py`, `depth.py` - page geometry: sizes, scale, offsets, where the ink
  bounding box lands on the page
- `solve.py`, `solve2.py`, `solve8.py`, `register.py`, `anchor.py` - fitting the
  transform between device pixels and page points
- `raster.py`, `rastergen.py`, `rast3.py` … `rast8.py`, `rastdots.py`,
  `rastnb.py`, `rastone.py` - render pages to pixels and compare them
- `verify.py`, `verify4.py`, `verify5.py`, `seqcheck.py`, `textcheck.py`,
  `pdfcheck.py`, `bigcheck.py`, `boxcheck.py`, `cmp.py`, `lezcmp.py` -
  assertions over exported PDFs: page counts, size runs, which pages carry text
- `prep.py`, `gentarget.py`, `sheet.py`, `full8.py`, `p3.py`, `v7.py` - fixture
  preparation
- `fmtboth.sh`, `fmtcheck.sh` - run `gofmt` over CRLF sources without rewriting
  them in place
