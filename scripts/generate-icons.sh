#!/bin/sh
# Raster variants all come from the same high-contrast framed Folio mark.
set -eu
cd "$(dirname "$0")/.."
python3 - <<'PYICONS'
from pathlib import Path
import copy,xml.etree.ElementTree as ET
ET.register_namespace('', 'http://www.w3.org/2000/svg')
ns='{http://www.w3.org/2000/svg}'
mark=ET.parse('web/public/brand-mark.svg').getroot()
icon=ET.Element(ns+'svg', {'viewBox':'0 0 64 64'})
ET.SubElement(icon,ns+'rect', {'x':'1','y':'1','width':'62','height':'62','rx':'14','fill':'#18191c','stroke':'#69717e','stroke-width':'1'})
g=ET.SubElement(icon,ns+'g', {'transform':'translate(10 10) scale(1.375)'})
for i,path in enumerate(mark):
 node=copy.deepcopy(path);node.set('fill','#f4f6fa' if i==0 else '#9aa7bc');g.append(node)
Path('web/public/favicon.svg').write_text(ET.tostring(icon,encoding='unicode')+'\n')
PYICONS
for size in 180 192 512; do
 output="web/public/icon-$size.png"
 if [ "$size" = 180 ]; then output=web/public/apple-touch-icon.png; fi
 magick -background none -density 768 web/public/favicon.svg -resize "${size}x${size}" -strip "$output"
done
magick -background none -density 768 web/public/favicon.svg -define icon:auto-resize=48,32,16 web/public/favicon.ico
