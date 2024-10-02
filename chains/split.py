import re
import sys

def split_yaml_content(content):
    # Split the content into individual chain configurations
    chains = re.split(r'\n(?=# [A-Za-z])', content)
    
    for chain in chains:
        # Extract the name from the chain configuration
        name_match = re.search(r'name: (\w+)', chain)
        if name_match:
            name = name_match.group(1)
            
            # Write the YAML content to a file
            with open(f"{name}.yaml", 'w') as file:
                file.write(chain.strip())
            print(f"Created {name}.yaml")

# Read the content from the file
try:
    with open('01_chains.yaml', 'r') as file:
        content = file.read()
except FileNotFoundError:
    print("Error: 'paste.txt' file not found. Make sure it's in the same directory as this script.")
    sys.exit(1)

# Try to find and extract the YAML content
yaml_match = re.search(r'<document_content>(.*?)</document_content>', content, re.DOTALL)

if yaml_match:
    yaml_content = yaml_match.group(1)
    # Split and create individual YAML files
    split_yaml_content(yaml_content)
    print("YAML files have been created successfully!")
else:
    # If <document_content> tags are not found, process the entire file
    print("Warning: <document_content> tags not found. Processing the entire file content.")
    split_yaml_content(content)

print("Script execution completed.")