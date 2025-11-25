OUT=${1%/}
cp "$OUT/config.yaml" /tmp/sigpc/
rm -rf "$OUT"
mv /tmp/sigpc "$OUT"
python3.14 -m venv "$OUT/.venv"
cd "$OUT" || exit
source "$OUT/.venv/bin/activate"
pip install -r requirements.txt
rm -r "$OUT/.scripts"
