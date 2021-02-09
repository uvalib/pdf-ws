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

function create_section ()
{
	mode="$1"
	data="$2"
	file="$3"

	touch "$file"

	if [ "$data" = "" ]; then
		echo "0"
		return
	fi

	case $mode in
		text )
			convert \
				-size "${capwidth}x" \
				-gravity center \
				-fill black \
				-background "$bkg" \
				-font "$font" \
				-pointsize "$pointreg" \
				-page "+${capmargin}+${yoffset}" \
				caption:"${data}" \
				"$file"
			;;

		logo )
			convert \
				-page "+${logoinset}+${yoffset}" \
				"$data" \
				logo.miff
			;;
	esac

	if [ ! -f "$file" ]; then
		echo "0"
		return
	fi

	identify -format "%h" "$file"
}

function create_cover_image ()
{
	echo "creating cover page..."

	width="924"
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

	# top/bottom margins
	topmargin="150"
	bottommargin="50"
	yoffset="$topmargin"

	ylast="$(create_section text "$header" header.miff)"
	(( yoffset += 100 + "$ylast" ))

	ylast="$(create_section logo "$logo" logo.miff)"
	(( yoffset += 100 + "$ylast" ))

	ylast="$(create_section text "$title" title.miff)"
	(( yoffset += "$ylast" ))

	ylast="$(create_section text "$author" author.miff)"
	(( yoffset += 100 + "$ylast" ))

	ylast="$(create_section text "$footer" footer.miff)"
	(( yoffset += "$bottommargin" + "$ylast" ))

	# grow the page if necessary
	[ "$yoffset" -gt "$height" ] && height="$yoffset"

	cat header.miff logo.miff title.miff author.miff footer.miff \
		| convert -size "${width}x${height}" xc:white - -flatten cover.png

	rm -f *.miff
}

function create_partial_pdfs ()
{
	echo "processing images..."

	numimages="$#"

	get_num_chunks "$numimages" "$numimagesperpdf"

	# one awk script to scare them all
	read -a hstats < <(identify "$@" 2>/dev/null | awk '
BEGIN {
	sum = 0
	sumsquares = 0
}

{
	# main loop: collect image heights to calculate mean and stdandard deviation

	# parse height from identify output.  example:
	# filename.jpg JPEG 2656x3749 2656x3749+0+0 8-bit sRGB 1.31518MiB 0.000u 0:00.000
	split($3, wh, "x")
	h = wh[2]

	sum += h
	sumsquares += h^2

	heights[NR] = h
}

END {
	# calculate image height limit based on mean + 2 standard deviations
	mean = sum / NR
	stddev = sqrt(sumsquares / NR - mean^2)
	limit = int(mean + 2 * stddev)

	# determine max image height that does not exceed limit
	maxheight = 0
	for (h in heights)
		if (h > maxheight && h <= limit)
			maxheight = h

	# books seem to hover just under 4000 pixels, while
	# newspapers and maps are closer to 6000 pixels.
	# set a different dpi for each case, using
	# the midpoint as a cutoff.
	dpi = 150
	if (maxheight > 5000)
		dpi = 300

	# set image size based on dpi
	inches = 11
	maxheight = inches * dpi

	print maxheight, dpi
}')

	hmax="${hstats[0]}"
	echo "height: ${hmax}"

	hdpi="${hstats[1]}"
	echo "dpi: ${hdpi}"

	for ((i=1;i<="$chunks";i++)); do
		ndx="$(expr \( \( "$i" - 1 \) \* "$numimagesperpdf" \) + 1)"
		end="$(expr "$ndx" + "$numimagesperpdf" - 1)"
		[ "$end" -gt "$numimages" ] && end="$numimages"
		len="$(expr "$end" - "$ndx" + 1)"

		get_next_pdf
		pdfs+=("$pdf")

		printf "[%3d/%3d] converting %3d images (%3d-%3d) into pdf: [%s]\n" "$i" "$chunks" "$len" "$ndx" "$end" "$pdf"

		convert -resize "x${hmax}" -density "$hdpi" "${@:$ndx:$len}" "$pdf"
	done
}

function merge_partial_pdfs ()
{
	echo "merging ${#pdfs[@]} pdfs into pdf: [$outpdf]"

	basepdf="$(basename "$outpdf")"
	outtitle="${basepdf/.pdf/}"

	gs \
		-q \
		-dBATCH \
		-dNOPAUSE \
		-sDEVICE=pdfwrite \
		-sOutputFile="$outpdf" \
		"${pdfs[@]}" \
		-c "[ /Title (${outtitle}) /DOCINFO pdfmark"
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
cover="n"
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
		-c ) cover="y"; shift ;;
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

# change to working directory
workdir="$(dirname "$outpdf")"
cd "$workdir" || die "could not change to directory: [$workdir]"

# now generate the pdf with optional cover page:

if [ "$cover" = "y" ]; then
	# validate arguments
	[ ! -f "$logo" ] && die "logo file does not exist: [$logo]"

	for var in header title footer; do
		val="${!var}"
		[ "$val" = "" ] && die "missing $var: [$val]"
	done

	create_cover_image

	create_partial_pdfs "cover.png" "$@"
else
	# no cover page
	create_partial_pdfs "$@"
fi

merge_partial_pdfs

do_cleanup

exit 0
