# Document identifiers in this file were redacted. Substitute a real
# rmfakecloud document id for the DOCUMENT-ID placeholders before running.
from pypdf import PdfReader, PdfWriter
d='DOCUMENT-ID'
for src, out, idx in ((f'/check/annot-{d}.pdf','/check/ink.pdf',0), (f'/check/payload-{d}.pdf','/check/target.pdf',0)):
    r=PdfReader(src); w=PdfWriter(); w.add_page(r.pages[idx])
    with open(out,'wb') as f: w.write(f)
    p=PdfReader(out).pages[0]
    print(out, round(float(p.mediabox.width),1), 'x', round(float(p.mediabox.height),1))
