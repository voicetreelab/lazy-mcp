This folder shows a full end to end example with an example config

1. First, we run the generate structure command with `go run ./structure_generator/cmd/main.go -config ./end_to_end_example/config.json -output ./end_to_end_example/initial_generate`

2. The user comes in and configures things as they expect. First, copty the folder into `user_input`. I split all the servers into sub-folders of tools. In serena, edit tools and some other top-level tools that might be used a lot. Note, you don't need to make all the files yourself. You just need to drag-and-drop each tool into a folder to categorise it, run step 3 and all the files get generated for you to edit.

3. Run `go run ./structure_generator/cmd/main.go -regenerate -output ./end_to_end_example/user_input`. This regenerates the necessary files and leaves some default overviews. The user can then come through and do their own editing to make the overviews as explanatory as they would like.

4. Now we can run the server!