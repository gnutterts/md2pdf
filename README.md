# md2pdf

`md2pdf` zet Markdown om naar PDF. De opdracht kent drie manieren van werken:

- Eén bestand naar één PDF: `md2pdf tmp/README.md`
- Een map samengevoegd naar één PDF: `md2pdf tmp/wiki`
- Een PDF per bestand in een map: `md2pdf --los tmp/wiki`

| Vlag | Betekenis |
|---|---|
| `-o <pad>` | Uitvoerbestand, of bij `--los` de uitvoermap |
| `--los`, `-l` | Maak een PDF per Markdown-bestand in een map |
| `--mermaid <pad>` | Pad naar de Mermaid-renderer |
| `--version` | Druk de versie af |
| `-h`, `--help` | Druk het gebruik af |

## Waar de PDF landt

Zonder `-o` komt de uitvoer naast de invoer te staan:

| Invoer | Uitvoer |
|---|---|
| `md2pdf tmp/README.md` | `tmp/README.pdf` |
| `md2pdf tmp/wiki` | `tmp/wiki.pdf`, naast de map |
| `md2pdf --los tmp/wiki` | een PDF naast elk bronbestand, in `tmp/wiki/` |

Met `-o` is het pad een bestandsnaam bij de eerste twee modi en een map bij `--los`. Een `-o` die
daarmee in tegenspraak is — een bestaande map waar een bestandsnaam hoort, of een bestaand bestand
waar een map hoort — geeft een foutmelding en exitcode 1.

Bij een map worden alle `.md`-bestanden in die map zelf meegenomen, niet die in submappen, in
alfabetische volgorde op bestandsnaam. Namen die met `_` beginnen worden overgeslagen; wie zo'n
bestand rechtstreeks als argument geeft, krijgt het wél verwerkt.

Bij de samengevoegde modus begint elk bronbestand op een nieuwe pagina. Bij `--los` wordt een
uitvoermap die nog niet bestaat aangemaakt.

## Wat er in de PDF terechtkomt

Koppen, alinea's, vet, cursief, inline code, links, geneste en genummerde lijsten, codeblokken,
blokcitaten, thematische breuken en tabellen. Tabellen krijgen een vette kopregel, kolommen op
eigen breedte en een herhaalde kopregel als ze over pagina's lopen.

De tekst wordt gezet in de base-14 fonts van PDF, die coderen in cp1252. Tekens daarbuiten worden
vertaald waar dat kan — `→` wordt `->` — en anders vervangen door een vraagteken.

Mermaid-blokken worden standaard gerenderd met `mmdc`. Kies een andere renderer met
`--mermaid <pad>` of `MD2PDF_MERMAID`; `uit` schakelt het renderen uit. Als de renderer ontbreekt of
faalt, verschijnt een waarschuwing op stderr en blijft het diagram als codeblok zichtbaar.

## Bekende beperkingen

- Mermaid-diagrammen vereisen een werkende externe renderer.
- Inline-opmaak binnen een tabelcel wordt afgevlakt tot platte tekst.
- Links zijn klikbaar maar niet zichtbaar onderscheiden van gewone tekst.
- Coderegels die breder zijn dan de tekstkolom worden afgekapt, niet afgebroken.
