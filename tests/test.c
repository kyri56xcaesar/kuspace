#include <stdio.h>
#include <ctype.h>


int main() {

    FILE *fp;
    fp = fopen("tests/input/job.in", "r");
    if (fp == NULL) {
        perror("Error opening input file");
        return 1;
    }

    FILE *out_fp = fopen("tests/output/job_c.out", "w");
    if (out_fp == NULL) {
        perror("Error opening output file");
        fclose(fp);
        return 1;
    }

    int ch;
    while ((ch = fgetc(fp)) != EOF) {
        fputc(toupper(ch), out_fp);
    }

    fclose(fp);
    fclose(out_fp);
}
