# md2pdf

`md2pdf` zet Markdown in een volgende mijlpaal om naar PDF. De opdracht ondersteunt drie invoermodi:

- Eén bestand naar één PDF: `md2pdf tmp/README.md`
- Een map samengevoegd naar één PDF: `md2pdf tmp/wiki`
- Een PDF per bestand in een map: `md2pdf --los tmp/wiki`

| Vlag | Betekenis |
|---|---|
| `-o <pad>` | Uitvoerbestand, of bij `--los` de uitvoermap |
| `--los`, `-l` | Maak een PDF per Markdown-bestand in een map |
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
alfabetische volgorde op bestandsnaam. Dat geldt ook voor wiki-navigatiebestanden als `_Footer.md`
en `_Sidebar.md`: ze worden niet apart behandeld.

Het renderen volgt in een volgende mijlpaal.
