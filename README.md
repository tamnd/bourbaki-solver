# bourbaki-solver

Go toolchain that turns Bourbaki PDFs into a tagged Markdown corpus, translates it, and solves the exercises.

The corpus it produces lives in [tamnd/bourbaki](https://github.com/tamnd/bourbaki). This repo is code and specs.

## What it does

```
PDF ──┬─ pdftotext -layout ──────────────┐
      │  (born digital: Algebra VIII)    │
      │                                  ├─ page files ─ assemble ─ split ─ tag ─ Markdown
      └─ pdftoppm ─ vision OCR fleet ────┘                                        │
         (scans: Algebra I-III, IV-VII)                                           │
                                                          translate (vi/zh/ja) ───┤
                                                          solve exercises ────────┘
```

Eight chapters of *Algebra*, 1699 pages. Two of the three volumes are 600 dpi JBIG2 scans whose legacy text layer is unusable for mathematics, so they go through vision OCR. The 2023 volume has a real text layer and extracts natively.

## Install

```sh
go install github.com/tamnd/bourbaki-solver/cmd/bourbaki@latest
```

Needs poppler for `pdfinfo`, `pdftotext`, `pdftoppm`, `pdfimages` and `pdffonts`:

```sh
brew install poppler
```

## Use

```sh
export BOURBAKI_CORPUS=$HOME/github/tamnd/bourbaki

bourbaki books add "pdf/en/Algebra Chapter 8 (2023, Springer Nature).pdf" --id alg-viii
bourbaki pagemap build --book alg-viii
bourbaki extract --book alg-viii
bourbaki assemble --book alg-viii
bourbaki split --book alg-viii --force --sync
bourbaki tags assign && bourbaki tags verify
bourbaki audit --report reports/audit.md
```

`bourbaki --help` lists everything else: `fleet`, `ocr`, `queue`, `refs`, `translate`, `solve`, `eval`, `report`, `publish`.

## The fleet

Model calls go through [tamnd/chatgpt-tool](https://github.com/tamnd/chatgpt-tool) running on a few hosts. The listener is loopback only, so text stages talk to it over SSH tunnels and OCR is driven over SSH directly, because the proxy takes no image input over HTTP.

A round trip is around 150 seconds. That single number shapes most of the design: work is a durable on disk queue with leases, batches are large, every stage is resumable, and nothing assumes API latency. Kill the process mid run and start it again, it picks up where it stopped.

Host names, keys and the design notes stay out of this repo. The milestones are tracked as issues.

## Layout

```
cmd/bourbaki      CLI
corpus            labels, tags, front matter, corpus model
pdfsrc            poppler wrappers
pagemap           printed page label to PDF page
extract           native text extraction
fleet             SSH, tunnels, routing, queue
api               chat client
assemble          pages to sections
share             public ChatGPT share pages, read whole over plain HTTP
translate         vi, zh, ja
solve             exercise solver and verifier
audit             corpus checks
```

## Licence

MIT for the code. The corpus it builds is derived from copyrighted material and is for personal study, see the licence in the corpus repo.
