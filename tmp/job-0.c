#include <stdio.h>
#define BUFFER_SIZE 1024

void run(char *data) {for (int i = 0; data[i] != '\0'; i++) {data[i] = toupper((unsigned char)data[i]);}}
int main() {
	FILE *in_fp, *out_fp;
	in_fp = fopen("/input/test_data.txt", "r");
	if (in_fp == NULL) {
		perror("Error opening input file");
		return 1;
	}
	out_fp = fopen("/output/test_c_data_capital.txt", "w");
	if (out_fp == NULL) {
		perror("Error opening output file");
		return 1;
	}
	char buffer[BUFFER_SIZE];
 	while (fgets(buffer, BUFFER_SIZE, in_fp) != NULL) {
    	run(buffer);         // Convert to uppercase
    	fputs(buffer, out_fp);  // Write to output file
	}
	fclose(in_fp);
	fclose(out_fp);
	return 0;
}