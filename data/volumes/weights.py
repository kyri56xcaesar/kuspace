"""
Engineer: Mylonakis Manolis
Description: Generates grid-like entries to configure the CA neighborhood
			 alongside the respective widgets. Run Simulation and Apply Configurations
			 buttons are also included.
"""

# ============================== IMPORTS ============================== #
# My Files
from run_sim import *
# GUI
from tkinter import *
from tkinter import ttk
# Maths (Linspace)
import numpy as np
from threading import Thread
import time
from threading import Thread # To Run simulation on the background and avoid program crashing
# Global Scope
TOTAL_ENTRIES = 29*29

# ============================== WEIGHTS FRAME / WIDGETS ============================== #
class Weights(ttk.Frame):
	"""
	Class attributes are shared between all instances contrary to instance attributes.
	We create one instace of the current class, so it won't be an issue.
	This way, we avoid passing/declaring all of these attributes into the constructor.
	"""
	# ====================== CLASS ATRIBUTES ====================== #
	# =========== Widgets =========== #
	weights_entries = []			  # Entries for weights
	diam_entry = None 				  # Entry for diamater
	set_button = None 				  # Set button for diameter
	menu_neigh_type = None 			  # Neighborhood type drop down menu
	add_weights_checkbox = None 	  # Check box next to Is Weighted ?
	add_entry = None 				  # Entry: Add to every cell 
	decr_button = None 				  # Decrement button next to Add Entry
	incr_button = None 				  # Increment button next to Add Entry
	add_button = None 				  # Add button next to incr_button.
	except_non_zeros_checkbox = None  # Except non-zeros values checkbox
	except_zeros_checkbox = None	  # Except zeros checkbox
	enable_button = None 			  # Mirror Mode Enable
	shift_center_checkbox = None      # Shift central point checkbox.
	done_button = None 				  # Done button for mirror mode.
	apply_button = None 			  # Apply Configurations button.
	error_msg_label = None            # To print error mesages, below the weights' entries.
	
	# =========== Extra Variables =========== #
	entry_vars = []      				   # To trace changes in Entries.
	xy_coordinates = []	 				   # Keeps a list of (x, y) tuples that correspond each weight entry to a coordinate.
	diameter = 29 		 				   # Diameter that has been set.		
	radius = 14          				   # Radius that has been set.
	neigh_type = "Moore" 				   # Type of neighborhood. Is being updated when a type is selected.
	is_weighted = 0 	 				   # Checkbox's value. If is weightes is set to 1.
	is_mirror_enabled = False 			   # Indicates us whether Mirror mode is enabled or not
	x0, y0 = (0, 0)						   # Central point of the entries.
	is_center_shifted = None    		   # Variable/Object that provided in checkbox "Shift Central Cell".
	final_neighborhood = [0]*TOTAL_ENTRIES # Neighborhood will always be 29x29.
	
	# Init current Frame.
	def __init__(self, parent):
		
		super().__init__(parent)
		
		# Init xy_coordinates list of tuples (x,y)
		self.init_coordinates()

		# Place Frame of Weights. Contains grid-like entries, buttons, etc.
		self.place(relx = 0.563, rely=0, relheight = 0.9, relwidth=0.6)
		# Place Grid-Like Entries.
		self.create_grid_like_entries()
		# Rest Widgets that add further functionality for configuring the weights.
		self.weights_functionalities(parent)

		# Apply Setting button interacts with Run Simulation button.
		# So we define both of them in this module/file.
		self.run_button = ttk.Button(parent, text ="Run Simulation", command=lambda:[self.run_simulation_button_pressed(parent)])
		self.run_button.pack(side='bottom', anchor='se', padx=3, pady=3)

		# Label to print Error Messages. Create a new frame for it.
		err_msg_frame = ttk.Frame(parent)
		err_msg_frame.place(relx=0.563, rely=0.91, relheight=0.09, relwidth=0.37)
		self.error_msg_label = Label(err_msg_frame, text= "", foreground="red", font = ('Romans 12 bold'), justify=LEFT)
		self.error_msg_label.pack(anchor='c', side=LEFT)

		# Disable widgest on init
		self.weights_disable_widgets()

		

	# Place 29x29 Entries for inserting neighborhood's weights.
	def create_grid_like_entries(self):
		# Use linspace to equally distribute entries.
		row_linspace = np.linspace(0.00, 0.963, 29, dtype=float)
		col_linspace = np.linspace(0.00, 0.7, 29, dtype=float)
		loops = 1 # Loops counter to find central entry.
		# Place entries according to the values of linspace
		for rows in row_linspace:
			for cols in col_linspace:
				entry_var = StringVar()
				weight_entry = Entry(self, width=3, borderwidth=2, font=('Romans 10 bold'), justify=CENTER, textvariable=entry_var)			
				weight_entry.insert(0, "1")
				
				# Center cell with green background
				if loops == np.ceil((TOTAL_ENTRIES)/2):
					weight_entry.config(bg="#50FFA0")
				else: # Blue background
					weight_entry.config(bg="#0090FF")

				weight_entry.place(relx=cols, rely=rows)
				# Keep a list of created entries.
				self.weights_entries.append(weight_entry)
				self.entry_vars.append(entry_var)
				loops += 1

		# To trace changes in an entry
		for i in range(TOTAL_ENTRIES):
			x, y = self.xy_coordinates[i]
			self.entry_vars[i].trace("w", lambda *_, entry=self.weights_entries[i], x=x, y=y: self.entry_triggered(*_, entry=entry, x=x, y=y))

	def weights_functionalities(self, parent):

		# We create a new Frame in parent window to place widgets in it.
		frame = ttk.Frame(parent)
		frame.place(relx=0.4, rely=0.6, relheight=0.395, relwidth=0.16)

		# Header
		header = Label(frame, text="CONFIGURE WEIGHTS", font=('Romans 10 bold'))
		header.place(relx=0.23, rely=0)

		# ============= NEIGHBORHOOD DIAMETER ============= #
		# Neighborhood Diameter Label
		diam_label = Label(frame, text="- Neighborhood Diameter:", font=('Romans 10'))
		diam_label.place(relx=0.062, rely=0.1)		
		# Diameter Entry
		self.diam_entry = Entry(frame, width=3, font=('Romans 10 bold'), justify=CENTER)
		self.diam_entry.place(relx=0.675, rely=0.1)
		# Diameter Set Button
		self.set_button = ttk.Button(frame, text="Set", width=5, command=lambda:[self.set_diameter_pressed()])
		self.set_button.place(relx=0.82, rely=0.095)
		# Diameter Info Label
		info_diameter = Label(frame, text="Diameter is an odd number in range [3,29]", fg="#FF0000")
		info_diameter.place(relx=0.09, rely=0.18)

		# ============= NEIGHBORHOOD TYPE ============= #
		# Label
		types_label = ttk.Label(frame, text="- Neighborhood Type:", font=('Romans 10'))
		types_label.place(relx=0.062, rely=0.27)
		# Drop down menu for weight types.
		selected_neigh_type = StringVar(frame) # To get what is clicked
		types = ("Moore", "Moore", "von Neumann", "Circular", "L2/Euclidean", "Cherckerboard", "Cherckerboard'", "Hash", "Cross", "Saltire", "Star")
		selected_neigh_type.set(types[0]) # type[0] default, the rest in menu.
		self.menu_neigh_type = ttk.OptionMenu(frame, selected_neigh_type, *types, command=self.update_neighborhood_type)
		self.menu_neigh_type.place(relx=0.56, rely=0.27)
		
		# Add Weights Label
		dash_label = ttk.Label(frame, text="- ", font=('Romans 10'))
		dash_label.place(relx=0.062, rely=0.35)
		# Add Weights Checkbox.
		checkbox_var1 = IntVar()
		self.add_weights_checkbox = ttk.Checkbutton(frame, text="Add Weights", variable=checkbox_var1, command=lambda:[self.update_is_weighted(checkbox_var1)])
		self.add_weights_checkbox.place(relx=0.1, rely=0.35)

		# ============= ADD OPERATION ============= #
		# Label
		add_label_1 = ttk.Label(frame, text="- Add to every cell", font=('Romans 10'))
		add_label_1.place(relx=0.062, rely=0.44)
		# Entry
		self.add_entry = ttk.Entry(frame, width=3, font=('Romans 10 bold'), justify=CENTER)
		self.add_entry.place(relx=0.5, rely=0.435)
		self.add_entry.insert(0, 0)
		
		# Decrement button.
		self.decr_button = ttk.Button(frame, text="-", width=1, command=lambda:[self.inc_or_dec_button_pressed(-1)])
		self.decr_button.place(relx=0.615, rely=0.427)
		
		# Increment button.
		self.incr_button = ttk.Button(frame, text="+", width=1, command=lambda:[self.inc_or_dec_button_pressed(+1)])
		self.incr_button.place(relx=0.67, rely=0.427)
		
		# Checkboxes variables.
		checkbox1_var = IntVar()
		checkbox2_var = IntVar()
		# Checkboxes.
		self.except_non_zeros_checkbox = ttk.Checkbutton(frame, text="Except non-zero values.", variable=checkbox1_var)
		self.except_non_zeros_checkbox.place(relx=0.09, rely=0.53)
		self.except_zeros_checkbox = ttk.Checkbutton(frame, text="Except zero values.", variable=checkbox2_var)
		self.except_zeros_checkbox.place(relx=0.09, rely=0.61)

		# Add button.
		self.add_button = ttk.Button(frame, text="Add", width=5, command=lambda:[self.add_to_every_entry(checkbox1_var, checkbox2_var)])
		self.add_button.place(relx=0.74, rely=0.427)
				
		# ============= MIRROR MODE ============= #
		# Label.
		mirror_label = ttk.Label(frame, text="- Mirror Mode -->")
		mirror_label.place(relx=0.062, rely=0.7)
		
		# Button Enable.
		self.enable_button = ttk.Button(frame, text="Enable", width=6, command=lambda:[self.enable_mirror_mode()])
		self.enable_button.place(relx=0.448, rely=0.69)
		
		# Label arrow.
		mirror_label = Label(frame, text="-->")
		mirror_label.place(relx=0.63, rely=0.695)
		
		# Button Done.
		self.done_button = ttk.Button(frame, text="Done", width=6, command=lambda:[self.mirror_the_entries()])
		self.done_button.place(relx=0.72, rely=0.69)

		# Shift center checkbox
		self.is_center_shifted = IntVar()
		# Checkboxes.
		self.shift_center_checkbox = ttk.Checkbutton(frame, text="Shift central point", variable=self.is_center_shifted, command=lambda:[self.shift_center(self.is_center_shifted)])
		self.shift_center_checkbox.place(relx=0.09, rely=0.78)

		# Apply Settings
		self.apply_button = ttk.Button(frame, text="Apply Configurations", command=lambda:[self.apply_configurations(parent)])
		self.apply_button.pack(side='bottom', anchor='se')

# ============================== WIDGETS INTERACTIONS / FUNCTIONALITIES ============================== #


	# ============= ENABLE / DISABLE WIDGETS ============= #
	
	# The two function below are called:
	# Enable: Only when no project was existed and new one is being created.
	# Disable: Only when all projects have been closed.

	# Enable
	def weights_enable_widgets(self):
		for weight_entry in self.weights_entries:
			weight_entry.config(state="normal")
			weight_entry.delete(0, END)
			weight_entry.insert(0, "1")

		self.diam_entry.config(state="normal")
		self.diam_entry.insert(0, "29")
		self.set_button.config(state="normal")
		self.menu_neigh_type.config(state="normal")
		self.add_weights_checkbox.config(state="normal")
		self.add_entry.config(state="normal")
		self.add_entry.insert(0, "0")
		self.decr_button.config(state="normal")
		self.incr_button.config(state="normal")
		self.add_button.config(state="normal")
		self.except_non_zeros_checkbox.config(state="normal")
		self.except_zeros_checkbox.config(state="normal")
		self.enable_button.config(state="normal")
		self.apply_button.config(state="normal")

	# Disable
	def weights_disable_widgets(self):
		for weight_entry in self.weights_entries:
			# If a project closes, some entries are already disabled and we can not delete their content, if we dont activate them first.
			weight_entry.config(state="normal")
			weight_entry.delete(0, END)
			weight_entry.config(state="disabled")

		self.diam_entry.delete(0, END)
		self.diam_entry.config(state="disabled")
		self.set_button.config(state="disabled")
		self.menu_neigh_type.config(state="disabled")
		self.add_weights_checkbox.config(state="disabled")		
		self.add_entry.delete(0, END)
		self.add_entry.config(state="disabled")
		self.decr_button.config(state="disabled")
		self.incr_button.config(state="disabled")
		self.add_button.config(state="disabled")
		self.except_non_zeros_checkbox.config(state="disabled")
		self.except_zeros_checkbox.config(state="disabled")
		self.enable_button.config(state="disabled")
		self.shift_center_checkbox.config(state="disabled")
		self.done_button.config(state="disabled")
		self.apply_button.config(state="disabled")
		self.run_button.config(state="disabled")

	# Import weights in entries when user switches project.
	def import_weights_from_file(self, neighborhood):
		# 1st Line = NEIGHBORHOOD:DIAMETER
		self.diam_entry.delete(0, END)
		self.diam_entry.insert(0, neighborhood[0].strip().split(':')[1])

		# It will always be 29x29. Smaller neighborhoods are wrapped-arround with 0s.
		count = 0
		for i in range(1, 30): # For every row
			for w in neighborhood[i].strip().split(' '): # Weights are seperated with ' '
				self.weights_entries[count].delete(0, END)
				self.weights_entries[count].insert(0, w)
				count += 1
	
	# Each entry correspond to a (x,y) coordinate on x'x|y'y axis.
	# This way, we can draw a neighborhood according to its type.
	def init_coordinates(self):
		# Horizontically traverse entries of weights using (x,y) coordinates
		for y in range(14, -15, -1):
			for x in range(-14, 15):
				self.xy_coordinates.append((x,y))

	# Traces when an entry has been triggered.
	def entry_triggered(self, *args, entry, x, y):
		# Check the value of the entry and change its background colour respectively.
		# Zeros and non central ones with ligth pink.
		if entry.get() == "0" and (self.x0 != x or self.y0 != y):
			entry.config(bg="#D0D0FF")
		# Wrong input with red.
		elif not(entry.get().isdigit()):
			entry.config(bg="#FF6040")
		# Central entry with green.
		elif self.x0 == x and self.y0 == y:
			entry.config(bg="#50FFA0")
		else: # Others with blue.
			entry.config(bg="#0090FF")

		# Something changed. Disable run simulation button.
		self.run_button.config(state = "disabled")
		# Also clear error message.
		self.error_msg_label.config(foreground="red")
		self.error_msg_label.config(text="")

	# Set diameter button has been pressed.
	def set_diameter_pressed(self):
		# Something changed. Disable run simulation button.
		self.run_button.config(state = "disabled")
		# Also clear error message.
		self.error_msg_label.config(foreground="red")
		self.error_msg_label.config(text="")

		# Get entry's value.
		entry_val = self.diam_entry.get()

		# Check if is alpharithmetic or even or out of range [3,29]
		if not(entry_val.isdigit()) or int(entry_val)%2==0 or int(entry_val)>29 or int(entry_val)<3:
			self.diam_entry.config(background="#FF6040") # Red background
			self.diam_entry.delete(0, END)
			self.diam_entry.insert(0, "%d" % int(self.diameter))
			return
		
		# White background.
		self.diam_entry.config(background="#FFFFFF")
		# Update diameter and radius
		self.diameter = int(entry_val)
		self.radius = int((self.diameter-1)/2)
		# To enable or disable "Shift central point" checkbox.
		self.enable_disable_shift_center()
		# Shift the center since a new diameter has propebly been set.
		self.shift_center(self.is_center_shifted)
		# Active and disable entries according to radius or mirror mode.
		self.update_active_entries()		
		# Draw neighborhood
		self.draw_neighborhood()

	# Enables and disables entries according to the provided diameter.
	def update_active_entries(self):		
		for i in range(TOTAL_ENTRIES):
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# Count distance, square-shaped.
			distance = max(abs(x), abs(y))
			
			# If mirror mode is enabled, keep active only the entries in the 2nd quadrant.
			if self.is_mirror_enabled:
				if distance <= self.radius and x<=0 and y>=0:
					self.weights_entries[i].config(state="normal")
					self.weights_entries[i].delete(0, END)
					self.weights_entries[i].insert(0, "0")
				else:# disable
					self.weights_entries[i].delete(0, END)
					self.weights_entries[i].insert(0, 0)
					self.weights_entries[i].config(state="disabled")
			else: # Else Activate or disable according to radius.
				if distance <= self.radius:
					self.weights_entries[i].delete(0, END)
					self.weights_entries[i].insert(0, "0")
					self.weights_entries[i].config(state="normal")			
				else:# disable
					self.weights_entries[i].delete(0, END)
					self.weights_entries[i].insert(0, 0)
					self.weights_entries[i].config(state="disabled")

	# Weights Type Menu has been triggered.
	def update_neighborhood_type(self, selection):
		# Something changed. Disable run simulation button.
		self.run_button.config(state = "disabled")
		# Also clear error message.
		self.error_msg_label.config(foreground="red")
		self.error_msg_label.config(text="")

		# Update type.
		self.neigh_type = selection
		# Draw neighborhood.
		self.draw_neighborhood()

	# Is weighted checkbox triggered.
	def update_is_weighted(self, var):
		# Something changed. Disable run simulation button.
		self.run_button.config(state = "disabled")
		# Also clear error message.
		self.error_msg_label.config(foreground="red")
		self.error_msg_label.config(text="")	

		self.is_weighted = var.get()
		# Draw neighborhood.
		self.draw_neighborhood()

	# Draws the neighborhood according to its type, weights, etc.
	def draw_neighborhood(self):
		r = self.radius			
		# Check if central point has been shifted
		if self.x0 != 0: # Then, the radius is r/2.
			# Shift the central point.	
			# In this case, it will always be and even number due to the way that we enable and disable the "Shift Central Point" checkbox.
			r = self.radius/2

		if self.neigh_type == "Moore":
			self.draw_Moore(r)
		elif self.neigh_type == "von Neumann":
			self.draw_vonNeumann(r)
		elif self.neigh_type == "Circular":
			self.draw_Circular(r)
		elif self.neigh_type == "L2/Euclidean":
			self.draw_L2Euclidean(r)
		elif self.neigh_type == "Cherckerboard":
			self.draw_Cherckerboard(parity=0) # Even
		elif self.neigh_type == "Cherckerboard'":
			self.draw_Cherckerboard(parity=1) # Odd
		elif self.neigh_type == "Hash":
			self.draw_Hash()
		elif self.neigh_type == "Cross":
			self.draw_Cross()
		elif self.neigh_type == "Saltire":
			self.draw_Saltire()
		elif self.neigh_type == "Star":
			self.draw_Star()
		
		self.color_entries()
	
	# Set colors to entries according to their value
	def color_entries(self):
		for i in range(TOTAL_ENTRIES):
			# Get coordinates
			x,y = self.xy_coordinates[i]
			# Central entry with green.
			if self.x0 == x and self.y0 == y:
				self.weights_entries[i].config(bg="#50FFA0")
				continue

			# Entries with 0s light ping.
			if int(self.weights_entries[i].get()) == 0:
				self.weights_entries[i].config(bg="#D0D0FF")
				continue

			# Others with blue
			self.weights_entries[i].config(bg = "#0090FF")

# ==================== NEIGHBORHOOD TYPES ==================== #
	# Draw Moore neighborhood.
	def draw_Moore(self, r):
		# Moore is a square-shaped region.
		for i in range(TOTAL_ENTRIES):
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue
			
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# Value to insert in entry
			value = 1
			# If is weighted, add a value according to the distance from the central point.
			if self.is_weighted:
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			# It is a square-shaped region, whole neighborhood.
			self.weights_entries[i].delete(0, END)
			self.weights_entries[i].insert(0, "%d" % value)

	# Draw von Neumann neighborhood.
	def draw_vonNeumann(self, r):
		# Diamond-shaped region.
		for i in range(TOTAL_ENTRIES):			
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance.
			if self.is_weighted:
				# According to distance from the center.
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			# If it's not in the disired region, set zero.
			if abs(x-self.x0) + abs(y-self.y0) <= r:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")
				
				
	def draw_Circular(self, r):
		# Circle: x^2 + x^2 = r^2. Area in circle: x^2 + x^2 <= r^2
		# We have discrete values (not Real) so we use x^2 + x^2 <= r^2+1
		for i in range(TOTAL_ENTRIES):			
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			if pow(abs(x-self.x0),2) + pow(abs(y-self.y0),2) <= (pow(r,2) + 1):
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

	def draw_L2Euclidean(self, r):
		for i in range(TOTAL_ENTRIES):
			
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			# For Euclidean x^2 + x^2 <= r^2
			if pow(abs(x-self.x0),2) + pow(abs(y-self.y0),2) <= (pow(r,2)):
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

	# Draw a checkerboard.
	# Parity = 0, every odd entry.
	# Parity = 1, every even entry.
	def draw_Cherckerboard(self, parity):
		for i in range(TOTAL_ENTRIES):
			
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			# i starts from 0. The first entry is even.
			if (i+1) % 2 == parity:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

	# Draw Hash. 2 Horintal and 2 Verical Lines on either side of central cell
	def draw_Hash(self):
		for i in range(TOTAL_ENTRIES):
			
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			if x==self.x0-1 or x==self.x0+1 or y==self.y0+1 or y==self.y0-1:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

	def draw_Cross(self):
		for i in range(TOTAL_ENTRIES):
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			if x==self.x0 or y==self.y0:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

	def draw_Saltire(self):
		for i in range(TOTAL_ENTRIES):
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				x, y = self.xy_coordinates[i]
				value = max(abs(x-self.x0), abs(y-self.y0)) + 1

			# Think of it like y= ax + y0, where a is the gradient and y0 is where the line cuts the Y-Axis.
			if y==x+2*self.y0 or y==-x:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

	def draw_Star(self):
		for i in range(TOTAL_ENTRIES):
			# If it is a disabled entry, skip it.
			if self.weights_entries[i]["state"] == "disabled":
				continue

			# Value to insert in entry
			value = 1
			# Get coordinates for the i-th entry
			x, y = self.xy_coordinates[i]
			# If is weighted, add a value according to distance
			if self.is_weighted:
				value = max(abs(x), abs(y)) + 1

			if y==x+2*self.y0 or y==-x or x==self.x0 or y==self.y0:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "%d" % value)				
			else:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, "0")

# ==================== ADD OPERATION ==================== #

	# + or - buttons have been pressed. 
	# If "+" is pressed, then number = 1.
	# If "-" is pressed, then number = -1.
	def inc_or_dec_button_pressed(self, number):
		# Get value.
		value = self.add_entry.get()
		# Left strip sign, if entry's value is not a digits return.
		if not(value.lstrip("-+").isdigit()):
			return

		# Add the number.
		self.add_entry.delete(0, END)
		self.add_entry.insert(0, "%d" % (int(value) + number))


	# Add button has been pressed
	def add_to_every_entry(self, except_non_zeros_var, except_zeros_var):
		# Something changed. Disable run simulation button.
		self.run_button.config(state = "disabled")
		# Also clear error message.
		self.error_msg_label.config(foreground="red")
		self.error_msg_label.config(text="")	

		# Entry's value to add
		to_add = int(self.add_entry.get())

		for i in range(TOTAL_ENTRIES):

			# If it's a non zero value, skip it.
			if int(self.weights_entries[i].get()) != 0 and except_non_zeros_var.get() == 1:
				continue

			# If it contains zero and has been excluded, skip it.
			if int(self.weights_entries[i].get()) == 0 and except_zeros_var.get() == 1:
				continue

			# Otherwise add in the entry
			old_value = int(self.weights_entries[i].get())
			self.weights_entries[i].delete(0, END)
			self.weights_entries[i].insert(0, "%d"%(old_value + to_add))


# ==================== MIRROR MODE ==================== #
	
	# Mirror mode has been enables. Enable button has been pressed.
	def enable_mirror_mode(self):
		# Something changed. Disable run simulation button.
		self.run_button.config(state = "disabled")
		# Also clear error message.
		self.error_msg_label.config(foreground="red")
		self.error_msg_label.config(text="")	

		# Mirror mode is enabled
		self.is_mirror_enabled = True

		# Disable "Enable" Button
		self.enable_button.config(state="disabled")
		# Enable Done Button
		self.done_button.config(state="normal")
		
		# To Enable or Disable "Shift central point" checkbox.
		self.enable_disable_shift_center()
		
		# Keep enable the entries that belong in the 2nd quadrant.
		for i in range(TOTAL_ENTRIES):
			# Get coordinates
			x, y = self.xy_coordinates[i]
			# If it is not in 2nd quadrant, disable.
			if x>0 or y<0:
				self.weights_entries[i].delete(0, END)
				self.weights_entries[i].insert(0, 0)
				self.weights_entries[i].config(state="disabled")
	"""
	During mirror mode, we provide user with the option to shift the central point (0,0) to the center of the 2nd quandrant (-r/2, r/2)
	This is only work for odd, NxN, quadrant boxes. Otherwise, a central point can not be determined.
 	In other words, radius must be an even number
	"""
	# Enables or Disable "Shift central point" checkbox.
	def enable_disable_shift_center(self):
		# Only if Mirror Mode is enabled and radius is an even number.
		if self.is_mirror_enabled and self.radius%2 == 0:
			self.shift_center_checkbox.config(state="normal")
		else:
			self.is_center_shifted.set(0) # Uncheck and disable.
			self.shift_center_checkbox.config(state="disabled")

	# Shift the central point to the center of the second quadrant.
	# We have taken care to enable or disable the checkbox, only when the aforementioned criteria are met.
	def shift_center(self, var):
		# Retrieve the value of the checkbox
		is_checked = var.get()
		if is_checked:
			self.x0, self.y0 = ((-self.radius)/2, self.radius/2)
		else:
			self.x0, self.y0 = (0, 0)

		self.draw_neighborhood()
		# Place green color to the new center.
		#self.color_entries()


	"""
	End mirroring. Done button has been pressed. Concept:
	quadrant_values is a dictionary where values of the 2nd will be stored. 
	The key is a tuple of (x,y) coordinates of the respective entry.
	Since we do simple horizontal scan of the grid entries, only one for-loop is required.
	Braking the scanning into rows and quadrants, we have the following order:
		1:  1st row of 2nd quadrant.
		2:  1st row of 1st quadrant.
		3:  2nd row of 2nd quadrant.
		4:  2nd row of 1st quadrant.
		.
		.
		.
		27: 14th row of 2nd quandrant
		28: 14th row of 1st quadrant.
		29: 1st row of 3rd quadrant.
		30: 1st row of 4th quadrant.
		.
		.
		.
	The 1st quadrant is the reflection of 2nd quadrant over the Y'Y axe. When the 1st quadrant needs the information
	from the 2nd one, it will be already updated in the dictionary quadrant_values due to horizontal scan.
	So, only one for-loop can do the job. It is not required to pre-store the values of the 2nd quadrants.

	Reflections(Q=quandrant):
	2nd Q --> 1st Q : x1=-x2, y1= y2 
	2nd Q --> 3rd Q : x3= x2, y3=-y2 
	2nd Q --> 4th Q : x4=-x2, y4=-y2

	"""
	def mirror_the_entries(self):
		# Mirror mode is disabled
		self.is_mirror_enabled = False
		# To disable "Shift central point" checkbox
		self.enable_disable_shift_center()
		# Enable "Enable" Button
		self.enable_button.config(state="normal")
		# Disable Done Button
		self.done_button.config(state="disabled")

		# To store the values in the 2nd quadrant
		quadrant_values = {}

		# Mirror Entries
		for i in range(TOTAL_ENTRIES):
			# Get Coordinates.
			x,y = self.xy_coordinates[i]
			# Distance from center
			distance = max(abs(x), abs(y))

			# If out of radius, skip it
			if distance > self.radius:
				continue

			# It's the 2nd quadrant
			if x<=0 and y>=0:
				# Store the entry's value.
				quadrant_values[(x,y)] = int(self.weights_entries[i].get())
				continue

			if x>0 and y>=0: # 1st quadrant
				# Reflected x,y
				rx = -x
				ry = y
				# Get the respective value in the 2nd quadrant
				rvalue = quadrant_values[(rx,ry)]
			elif x<0 and y<0: # 3rd quadrant
				# Reflected x,y
				rx = x
				ry = -y
				# Get the respective value in the 2nd quadrant
				rvalue = quadrant_values[(rx,ry)]
			else: # 4th quadrant
				# Reflected x,y
				rx = -x
				ry = -y
				# Get the respective value in the 2nd quadrant
				rvalue = quadrant_values[(rx,ry)]

			# Enable entry and insert value.
			self.weights_entries[i].config(state="normal")
			self.weights_entries[i].delete(0, END)
			self.weights_entries[i].insert(0, rvalue)
		
		# Update new center and color it.
		self.x0, self.y0 = (0,0)
		self.color_entries()

# ====================== APPLY CONFIGURATIONS AND RUN SIMULATION ====================== #

	# Auto save configurations, when the user switches project.
	def auto_save_configs(self, parent, config_file):
		# To store the content of the file, line by line. This is the 1st line.
		file_content = ["*** PLEASE DO NOT EDIT. This file is auto-generated by the tool. ***\n"]
		# 2nd line: Number of states. strip() it, if user add space character by accident.
		file_content.append("States:%s\n" % parent.rules.entry_states.get().strip())
		# 3rd line: Grid Type
		file_content.append("Grid_Type:%s\n" % parent.rules.dd_menu_var.get())
		# 4th Line
		file_content.append("# Transition Rule\n")
		# Get Transition Rule from Text Editor.
		file_content.append(parent.rules.TR_editor.get("1.0", END).strip()+"\n_END_\n")
		# Add line NEIGHBORHOOD:DIAMETER
		file_content.append("NEIGHBORHOOD:%s\n" % self.diam_entry.get().strip())

		# 29x29 weights. One line contains 29 weights
		line_weights = ""
		for i in range(TOTAL_ENTRIES):
			# Every 29, switch line. Each weight is separated with a space char.
			if (i+1) % 29 == 0:
				# "... w\n"
				line_weights += self.weights_entries[i].get() + "\n"
				file_content.append(line_weights)
				line_weights = ""
			else:
				#"... w w "
				line_weights += self.weights_entries[i].get() + " "

		# Store in file.
		with open(config_file, "w") as file:
			file.writelines(file_content)
		file.close()


	"""
	Apply Configurations button has been pressed.
	Checks user's input, if hardware's specifications met.
	Stores the setting into the .config and enables the capability to run simulation
	"""
	def apply_configurations(self, parent):
		# Store configurations in .config file.
		self.auto_save_configs(parent, parent.prj_tree.config_path)
		
		# Check initial state.
		correct = parent.img_viewer.check_init_state(parent)
		if not correct: return

		# Check entry Total Screenshots.		
		correct = parent.rules.check_entry_total_screenshots()
		if not correct:
			self.error_msg_label.config(foreground="red")
			self.error_msg_label.config(text="Total Screenshots field contains incorrect value.\n It should be any positive integer greater or equal to 1.")
			return

		# Check entry Time Step.
		correct = parent.rules.check_entry_time_step()
		if not correct:
			self.error_msg_label.config(foreground="red")
			self.error_msg_label.config(text="Time step field contains incorrect value.\n It should be an integer in range [10, 16000].")
			return


		# Check Transtion rule
		correct, msg = parent.rules.transition_rule_analyzer()
		if not correct:
			self.error_msg_label.config(foreground="red")
			self.error_msg_label.config(text=msg)
			return
				

		# All checks passed.
		# Store final form of neighborhood.
		# For every weight entry.
		for i in range(TOTAL_ENTRIES):
			# Store each value as integer.
			self.final_neighborhood[i] = int(self.weights_entries[i].get())			

		# Input is corrent. Enable Run Simulation Button.
		self.run_button.config(state = "normal")
		self.error_msg_label.config(foreground="green")
		self.error_msg_label.config(text="Everything is fine. You may proceed to simulation !")


	# Run simulation button pressed.
	def run_simulation_button_pressed(self, parent):		
		# Thread 1: To run simulation in the background.
		thread_run = Thread(target=self.simulate, args=(parent, ))		
		# Thread 2: To pop up a window that takes priority over the main window.
		thread_pop_up = Thread(target=self.pop_up_win, args=(parent, ))		
		# Start threads
		thread_pop_up.start()		
		thread_run.start()
		
	# Spawn pop up window.
	pop_up_window = None
	def pop_up_win(self, parent):
		# Spawn a pop up window to take priority over the main.
		self.pop_up_window = parent.menu_bar.pop_up_window_func(parent, title="Simulating...", window_width=255, window_height=45, disable_toolbar=True)
		# Label
		label = Label(self.pop_up_window, text="Simulation is running. Please wait !", font = ('Romans 11 bold'))
		label.pack(anchor='nw', pady=11)

	# Run Simulation.
	def simulate(self, parent):
		curr_prj = parent.prj_tree.selected_prj # Selected project to store results.
		init_state = parent.img_viewer.img_arr # Initial state, the image array.
		# The name of the initial image. We use it to generation the names of the result appropriately.
		init_img_name = parent.rules.init_state_entry.get().replace(".bmp", "") # Also remove its data type.
		total_states = int(parent.rules.entry_states.get()) # Total number of states.
		cell_size = parent.rules.cell_size     # Cell size in bits.
		neighborhood = self.final_neighborhood # The neighborhood.
		total_extracts = int(parent.rules.total_screenshots_entry.get()) # Total number of extractions that the user requested.
		time_step = int(parent.rules.step_entry.get())
		bram_values = parent.rules.LUT_BRAM
		others_value = parent.rules.others
		addr_sel = parent.rules.addr_sel
        #AWS
		credentials_file = "aws_credentials.json"
		region = "eu-central-1"
		instance_id = "i-0f037e88180112bd0"
		bucket_name = "tucbucketca5"
		# Grid Type. If it is TOROIDAL, 1920x1080 grid values are extracted.
		# Otherwise, we expect 1920x(1080-28), because upper-most 14 and bottom-most 14 rows of the grid are set to 0. (radius = 14)
		# This is how the hardware treats these boundary conditions.
		# The TOROIDAL grid eliminates boundary conditions, since it is a priodically, infinite grid.
		grid_type = parent.rules.dd_menu_var.get() # Drop Down menu of grid type.
		if grid_type == "TOROIDAL":
			vertical_offset = 0
		else:
			vertical_offset = 28

		# Set the 0s in the init state.
		if vertical_offset != 0:
			for i in range(0,14):
				for j in range(0, 1920):
					init_state[i][j] = 0

			for i in range(1080-14,1080):
				for j in range(0, 1920):
					init_state[i][j] = 0

		list_of_extracted_grids = run_simulation(curr_prj, init_state, grid_type, cell_size, self.final_neighborhood, time_step, 
												 bram_values, others_value, addr_sel, total_extracts, vertical_offset,credentials_file,region,instance_id,bucket_name)
		
		# Full path of results. We are goind to replace *@* with the number of generations. 
		# We wish that user will use the sequence "*@*" to name a file.bmp. What are the odds ? :'-).
		results_path = curr_prj + '/' + init_img_name + "_result_*@*.bmp"
		
		print("Converting Arrays to Image...")
		curr_generation = time_step
		for i in range(0, total_extracts):
			img_out_path = results_path.replace('*@*', '%dGENS'%curr_generation)
			parent.img_viewer.array2img(list_of_extracted_grids, img_out_path, 0, total_states)
			# Update tree_elements dictionary with new images (the names).
			parent.prj_tree.tree_elements[curr_prj].append(img_out_path.split("/")[-1])			
			curr_generation += time_step

		# Update Project Tree
		parent.prj_tree.update_elements(parent)
		# Update paths of bmps and open them in Image viewer.
		parent.prj_tree.update_file_paths_of_curr_prj(parent)
		# Close already open images and open new ones.
		parent.img_viewer.close_images()
		parent.img_viewer.open_images(parent.prj_tree.list_bmp_paths)

		print("Done !")

		# Destroy Pop up  window.
		self.pop_up_window.destroy()
