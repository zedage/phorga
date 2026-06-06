Als nächstes möchte ich, dass es möglich ist, die sprache einzustellen. dies soll sich konkret auf die Namen der Monate 
beziehen, wie diese im Target bezeichnet werden. Es soll möglich sein, die Sprache auf Deutsch, Italienisch, Englisch und Französisch einzustellen. Die Standardeinstellung soll Englisch sein. Ich denke diese Einstellung sollte im config.yaml unter organizer.target.language erfolgen. Es könnte dann so aussehen:

```yamlorganizer:
  - target: /photos/organized
    targetStructure: "{year}/{month}"
    language: "de"   # de, it, en, fr
```

Eine andere Variante wäre, dass wir targetStructure so anpassen, dass die Sprache im Platzhalter selbst definiert, bzw. der Platzhalter in der gewünschten Sprache geschrieben wird. Also z.B. {month} für Englisch, {monat} für Deutsch, {mois} für Französisch und {mese} für Italienisch. Das hätte den Vorteil, dass die Sprache direkt im targetStructure definiert ist und nicht zusätzlich über eine separate Einstellung festgelegt werden muss. Es könnte dann so aussehen:

```yamlorganizer:
  - target: /photos/organized
    targetStructure: "{year}/{month}"   # for English
    targetStructure: "{year}/{monat}"   # for German
    targetStructure: "{year}/{mois}"    # for French
    targetStructure: "{year}/{mese}"    # for Italian
```

zusätzlich müsste das natürlich für Tag, und TagNummer und Monat MonatsName, Jahr auch entsprechend angepasst werden. Also z.B. {day} für Englisch, {tag} für Deutsch, {jour} für Französisch und {giorno} für Italienisch. Das gleiche gilt für Jahr: {year} für Englisch, {jahr} für Deutsch, {année} für Französisch und {anno} für Italienisch. was meinst du?

als Resultat möchte ich die Möglichkeit bieten, dass folgendes möglich ist:

```
.../photos/organized/2024/02
.../photos/organized/2024/Februar
.../photos/organized/2024/Febbraio
.../photos/organized/2024/Février
.../photos/organized/2024/02/15
.../photos/organized/2024/Februar/15
.../photos/organized/2024/Febbraio/15
.../photos/organized/2024/02-Februar/15-Montag
.../photos/organized/2024/02_Februar
```

Gross- Kleinschreibung, jeder soll frei sein, das so zu definieren wie er möchte. Was meinst du, was eine gute Konigurationsmöglichkeit dafür wäre?