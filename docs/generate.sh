#!/bin/sh
set -eu
dt="$(date -d 'now + 2 months' '+%Y-%m-%d')"
idx="index.html"
cd "$(cd "$(dirname "$0")"; pwd)" 
echo "<!DOCTYPE html><head><title>Example fares</title></head><body><ul>" >"$idx"
tmpl="<tr><td>{{.Price}}</td><td>{{.Day}}</td><td>{{.Destination.IATACode}}</td><td>{{.Airline}}</td></tr>"
for src in BUD VIE; do
  {
echo "<!DOCTYPE html><head><title>${src} @ ${dt}</title></head>"
echo "<body><pre>fly fares \"-template=$tmpl\" \"-origin=$src\" \"$dt\"</pre>"
echo "<table><thead><tr><th>Price</th><th>Day</th><th>Destination</th><th>Airline</th></tr></thead><tbody>"
fly fares "-template=$tmpl" "-origin=$src" "$dt"
echo "</tbody></table></body></html"
  } >"${src}.html" &
  echo "<li><a href=\"${src}.html\">${src}</li>"
done >>"$idx"
wait
echo "</ul></body></html>" >>"$idx"
