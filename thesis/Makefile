# makefile for compiling the latex recipe using pdflatex and biber 
#
# # supplementary ##
#
#

MAIN := main.tex 
PRES := thesis-presentation.tex 

.PHONY: main pres 

main:
	pdflatex ${MAIN} 
	biber ${MAIN%%.*}
	pdflatex ${MAIN}
	pdflatex ${MAIN} 

pres:
	pdflatex ${PRES}
	biber ${PRES%%.*}
	pdflatex ${PRES}
	pdflatex ${PRES}

clean:
	rm -rf *.aux *.bcf *.loa *.lof *.lot *.out *.xml *.gz *.toc *.nav *.snm 
