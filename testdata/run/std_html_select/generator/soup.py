import json
import sys

import soupsieve
from bs4 import BeautifulSoup

corpus = json.load(open(sys.argv[1]))
soups = [BeautifulSoup(d["html"], "html5lib") for d in corpus["docs"]]
elements = [soup.find_all(True) for soup in soups]
out = []
for selector in corpus["selectors"]:
    entry = {"selector": selector, "docs": []}
    try:
        compiled = soupsieve.compile(selector)
        for d, soup in enumerate(soups):
            ids = {id(e): i for i, e in enumerate(elements[d])}
            found = [ids[id(e)] for e in compiled.select(soup)]
            entry["docs"].append(found)
    except Exception as e:
        entry["error"] = f"{type(e).__name__}: {e}"
        entry["docs"] = None
    out.append(entry)
json.dump(out, open(sys.argv[2], "w"))
print(len(out), [len(e) for e in elements], soupsieve.__version__)
