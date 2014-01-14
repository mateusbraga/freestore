default: install

install:
	go install github.com/mateusbraga/freestore/pkg/view
	go install github.com/mateusbraga/freestore/pkg/client
	go install github.com/mateusbraga/freestore/pkg/server
	go install github.com/mateusbraga/freestore/cmd/freestored
	go install github.com/mateusbraga/freestore/cmd/freestore_client
	go install github.com/mateusbraga/freestore/cmd/freestore_client/freestore_client_measures

clean:
	go clean -i github.com/mateusbraga/freestore/pkg/view
	go clean -i github.com/mateusbraga/freestore/pkg/client
	go clean -i github.com/mateusbraga/freestore/pkg/server
	go clean -i github.com/mateusbraga/freestore/cmd/freestored
	go clean -i github.com/mateusbraga/freestore/cmd/freestore_client
	go clean -i github.com/mateusbraga/freestore/cmd/freestore_client/freestore_client_measures

upload:
	rsync -avz -f"- .git/" -f"+ *" --delete /run/media/mateus/Storage/arquivos/projetos/programar/freestore/ mateusbr@users.emulab.net:/proj/freestore/src/mateusbraga/freestore

getresult:
	rsync -avz -f"- go/" -f"+ *" mateusbr@pc299.emulab.net:/home/mateus/ /home/mateus/results/

