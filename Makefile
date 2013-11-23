default: install

install:
	go install mateusbraga/freestore/view
	go install mateusbraga/freestore/client
	go install mateusbraga/freestore/server
	go install mateusbraga/freestore/samples/client
	go install mateusbraga/freestore/samples/server

clean:
	go clean -i mateusbraga/freestore/view
	go clean -i mateusbraga/freestore/client
	go clean -i mateusbraga/freestore/server
	go clean -i mateusbraga/freestore/samples/client
	go clean -i mateusbraga/freestore/samples/server

upload:
	rsync -avz -f"- .git/" -f"+ *" --delete /run/media/mateus/Storage/arquivos/projetos/programar/freestore/ mateusbr@users.emulab.net:/proj/freestore/src/mateusbraga/freestore
