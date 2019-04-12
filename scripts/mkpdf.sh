#!/usr/bin/env bash

# merge OCR text files

# general arguments
outpdf=""
numimagesperpdf="50"

# cover page arguments
header=""
logo=""
title=""
author=""
footer=""

# internal variables
pdfs=()
count="0"

function die ()
{
	echo "error: $@"
	exit 1
}

function get_num_chunks ()
{
	local items="$1"
	local chunksize="$2"

	chunks="$(expr \( \( "$items" - 1 \) / "$chunksize" \) + 1)"

	echo "[$items] items will be split into [$chunks] chunks of size [$chunksize]"
}

function get_next_pdf ()
{
	((count++))

	pdf="$(printf "partial-%04d.pdf" "$count")"
}

function create_cover_image ()
{
	echo "creating cover page..."

	width="1024"
	height="1320"

	# captions with margins
	capmargin="100"
	capwidth="$(expr "$width" - 2 \* "$capmargin")"

	logowidth="$(identify -format "%w" "$logo")"
	logoinset="$(expr \( "$width" - "$logowidth" \) / 2)"

	pointreg="20"
	pointbig="30"

	font="Arial"
	#font="TimesNewRoman"

	bkg="none"

	yoffset="150"

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointreg" \
		-page "+${capmargin}+${yoffset}" caption:"${header}" header.miff
	ylast="$(identify -format "%h" header.miff)"
	(( yoffset += 100 + "$ylast" ))

	convert -page "+${logoinset}+${yoffset}" "$logo" logo.miff
	ylast="$(identify -format "%h" logo.miff)"
	(( yoffset += 100 + "$ylast" ))

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointbig" \
		-page "+${capmargin}+${yoffset}" caption:"${title}" title.miff
	ylast="$(identify -format "%h" title.miff)"
	(( yoffset += "$ylast" ))

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointreg" \
		-page "+${capmargin}+${yoffset}" caption:"${author}" author.miff
	ylast="$(identify -format "%h" author.miff)"
	(( yoffset += 100 + "$ylast" ))

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointreg" \
		-page "+${capmargin}+${yoffset}" caption:"${footer}" footer.miff

	cat header.miff logo.miff title.miff author.miff footer.miff \
		| convert -size "${width}x${height}" xc:white - -flatten cover.png

	rm -f *.miff
}

function create_partial_pdfs ()
{
	echo "processing images..."

	numimages="$#"

	get_num_chunks "$numimages" "$numimagesperpdf"

	# determine a reasonable maximum height for limiting oddly-shaped images such as spines
	maxheight="$(identify "$@" 2>/dev/null | awk '
BEGIN {
	limit = 1024 * 1.5;
	maxh = 0;
}
{
	split($3, wh, "x");
	h = wh[2];
	if (h < limit && h > maxh)
		maxh = h;
	}
END {
	print maxh;
}')"

	for ((i=1;i<="$chunks";i++)); do
		ndx="$(expr \( \( "$i" - 1 \) \* "$numimagesperpdf" \) + 1)"
		end="$(expr "$ndx" + "$numimagesperpdf" - 1)"
		[ "$end" -gt "$numimages" ] && end="$numimages"
		len="$(expr "$end" - "$ndx" + 1)"

		get_next_pdf
		pdfs+=("$pdf")

		printf "[%3d/%3d] converting %3d images (%3d-%3d) into pdf: [%s]\n" "$i" "$chunks" "$len" "$ndx" "$end" "$pdf"

		convert -resize "x${maxheight}>" "${@:$ndx:$len}" "$pdf"
	done
}

function merge_partial_pdfs ()
{
	echo "merging ${#pdfs[@]} pdfs into pdf: [$outpdf]"

	gs -dBATCH -dNOPAUSE -q -sDEVICE=pdfwrite -sOutputFile="$outpdf" "${pdfs[@]}"
}

function do_cleanup ()
{
	echo "cleaning up..."

	rm -f "${pdfs[@]}"
}

### parse command line

# general arguments
outpdf=""
numimagesperpdf="50"

# cover page arguments
header=""
logo=""
title=""
author=""
footer=""

while [ "$#" -gt "0" ]; do
	arg="$1"
	val="$2"

	case $arg in
		-a ) author="$val"; shift; shift ;;
		-f ) footer="$val"; shift; shift ;;
		-h ) header="$val"; shift; shift ;;
		-l ) logo="$val"; shift; shift ;;
		-n ) numimagesperpdf="$val"; shift; shift ;;
		-o ) outpdf="$val"; shift; shift ;;
		-t ) title="$val"; shift; shift ;;
		-- ) shift; break ;;
		-* ) die "unknown option: [$arg]" ;;
		 * ) break ;;
	esac
done

# validate arguments
[ ! -f "$logo" ] && die "logo file does not exist: [$logo]"
for var in header author title footer; do
	val="${!var}"
	[ "$val" = "" ] && die "missing $var: [$val]"
done

# change to working directory
workdir="$(dirname "$outpdf")"
cd "$workdir" || die "could not change to directory: [$workdir]"

# now generate the pdf:

create_cover_image

create_partial_pdfs "cover.png" "$@"

merge_partial_pdfs

do_cleanup

exit 0
