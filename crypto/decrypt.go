// Package crypto contenant les fonctions de chiffrement/déchiffrement.
package crypto

import "crypto/aes"
import "crypto/cipher"
import "errors"
import "os"
import "fmt"
import "io/ioutil"
import "strings"
import "sync"

import "github.com/Heisenberk/goshield/structure"

// DecryptBlocAES déchiffre 1 bloc input avec la clé key et la valeur initiale iv pour donner le bloc déchiffré. 
func DecryptBlocAES(iv []byte, key []byte, input []byte) ([]byte, error){

	// Résultat du chiffrement sera dans output.
	output := make([]byte, aes.BlockSize)

	// Si la taille de l'entrée est invalide on lance une erreur. 
	if len(input)%aes.BlockSize != 0 {
		return output, errors.New("- \033[31mFailure Decryption\033[0m : Taille du bloc invalide.")
	}

	// Preparation du bloc qui sera chiffré.
	block, err := aes.NewCipher(key)
	if err != nil {
		return output, errors.New("- \033[31mFailure Decryption\033[0m : Erreur lors du déchiffrement d'un bloc.")
	}

	// Chiffrement AES avec le mode opératoire CBC.
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(input, input)

	return input, nil
}

// DecryptFileAES déchiffre un fichier de chemin pathFile avec les données doc. 
func DecryptFileAES(pathFile string, doc *structure.Documents, channel chan error, wg *sync.WaitGroup){

	// synchronisation pour les autres goroutines.
	defer wg.Done()
	
	// ouverture du fichier à déchiffrer
	inputFile, err1 := os.Open(pathFile) 
	if err1 != nil {
		var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible d'ouvrir le fichier à déchiffrer "+pathFile+". "
		channel <- errors.New(texteError)
		return 
	}

	// renvoie une erreur si l'extension n'est pas la bonne
	if pathFile[(len(pathFile)-4):]!= ".gsh"{
		var texteError string = "- \033[31mFailure Decryption\033[0m : L'extension de "+pathFile+" est invalide (doit être \".gsh\"). "
		channel <- errors.New(texteError)
		return
	}

	// renvoie une erreur si la signature n'est pas correcte
	signature := make([]byte, 8)
    _, err2 := inputFile.Read(signature)
    if err2 != nil {
		var texteError string = "- \033[31mFailure Decryption\033[0m : Format du fichier à déchiffrer "+pathFile+" invalide. "
		channel <- errors.New(texteError)
		return
	}

    // lecture du salt et déduction de la clé
    salt := make([]byte, 15)
    _, err22 := inputFile.Read(salt)
    if err22 != nil {
		var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible de lire le salt du fichier chiffré "+pathFile+". "
		channel <- errors.New(texteError)
		return
	}
	doc.Salt=salt
	DeductHash(doc)

	// lecture de la valeur IV
	IV := make([]byte, 16)
	_, err23 := inputFile.Read(IV)
    if err23 != nil {
		var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible de lire la valeur d'initialisation du fichier chiffré "+pathFile+". "
		channel <- errors.New(texteError)
		return
	}

	// lecture de la taille du dernier bloc
	lengthTab := make([]byte, 1)
	_, err24 := inputFile.Read(lengthTab)
    if err24 != nil {
		var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible de lire la taille du dernier bloc du fichier chiffré "+pathFile+". "
		channel <- errors.New(texteError)
		return
	}

	stat, err2 := inputFile.Stat()
	if err2 != nil {
  		var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible d'interpréter le fichier à déchiffrer "+pathFile+". "
		channel <- errors.New(texteError)
		return
	}

	// on soustrait la taille de la signature (8) + le salt (15) + IV (16) + taille du dernier bloc (1)
	var division int = (int)((stat.Size()-8-15-16-1)/aes.BlockSize) 
	var iterations int = division
	if (int)(stat.Size()-8-15-16-1)%aes.BlockSize != 0 {
		var texteError string = "- \033[31mFailure Decryption\033[0m : Fichier" + pathFile +" non conforme pour le déchiffrement AES. "
		channel <- errors.New(texteError)
		return
	}

    // ouverture du fichier résultat
    var nameOutput string=pathFile[:(len(pathFile)-4)]
    outputFile, err3 := os.Create(nameOutput)
    if err3 != nil {
  		var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible d'écrire le fichier chiffré "+nameOutput+". "
		channel <- errors.New(texteError)
		return
	}

	input := make([]byte, 16)
	var cipherBlock []byte
	temp := make([]byte, 16)
	
	for i:=0 ; i<iterations ; i++ {

    	// si on est au tour i (i!=0), IV vaut le chiffré du tour i-1
    	if (i) != 0 {
    		IV = temp
    	}

		input =[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

		// lecture de chaque bloc de 16 octets
		_, err8 := inputFile.Read(input)
		if err8 != nil {
  			var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible de lire dans le fichier à déchiffrer "+pathFile+". "
			channel <- errors.New(texteError)
			return
		}

    	temp =[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		copy(temp, input)
		
		// déchiffrement de chaque bloc et écriture
		var err10 error
		cipherBlock, err10 = DecryptBlocAES(IV, doc.Hash, input)
		if err10 != nil {
			var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible de déchiffrer le fichier "+pathFile+". "
			channel <- errors.New(texteError)
			return
		}
		
		// dans le dernier bloc, il faut enlever les bits de padding qui ne sont pas dans le message initial.
		if i==(iterations-1) {
			if lengthTab[0]!= 0 {
				_, err11 := outputFile.Write(cipherBlock[:lengthTab[0]])
				if err11 != nil {
  					var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible d'écrire dans le fichier "+nameOutput+". "
					channel <- errors.New(texteError)
					return
				}
			}else {
				_, err12 := outputFile.Write(cipherBlock)
				if err12 != nil {
  					var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible d'écrire dans le fichier "+nameOutput+". "
					channel <- errors.New(texteError)
					return
				}
			}
			
			
		}else {
			_, err13 := outputFile.Write(cipherBlock)
			if err13 != nil {
  				var texteError string = "- \033[31mFailure Decryption\033[0m : Impossible d'écrire dans le fichier "+nameOutput+". "
				channel <- errors.New(texteError)
				return
			}
		}
	}

	// fermeture des fichiers. 
	inputFile.Close()
	outputFile.Close()

	var messageSuccess string = "- \033[32mSuccess Decryption\033[0m : "+pathFile+" : resultat dans le fichier "+nameOutput
    fmt.Println(messageSuccess)

	channel <- nil
	return
}

// DecryptFolder déchiffre le contenu d'un dossier de chemin path avec les données doc. 
func DecryptFolder (path string, d *structure.Documents) {

	// Permettra de synchroniser le chiffrement des fichiers contenus dans le dossier. 
	wgFolder := &sync.WaitGroup{}

    // Lecture du chemin à déchiffrer. 
   entries, err := ioutil.ReadDir(path)
    if err != nil {
        fmt.Println("- \033[31mFailure Decryption\033[0m : impossible d'ouvrir "+path)
    }

    // Comptage des futures goruntines à lancer. 
    var countFiles int = 0
    for _, entry := range entries {

        newPath := path+entry.Name()
        fi, err := os.Stat(newPath)
        valid := true
        if err != nil {
            fmt.Println("- \033[31mFailure Decryption\033[0m : "+newPath+" n'existe pas ")
            valid = false
        }

        // si l'élément du dossier existe. 
        if valid == true {

            mode := fi.Mode();

            if mode.IsRegular()== true {

                // si l'extension du fichier est différent de .gsh on peut chiffrer le fichier.
                if newPath[len(newPath)-4:]==".gsh"{
                    countFiles=countFiles+1
                }     
            }
        }
    }

    // Initialisation des goroutines
	wgFolder.Add(countFiles)
	channelFolder := make (chan error)

    // Déchiffrement de chaque élément du dossier. 
    for _, entry := range entries {

        newPath := path+entry.Name()
        fi, err := os.Stat(newPath)
        valid := true
        if err != nil {
            fmt.Println("- \033[31mFailure Decryption\033[0m : "+newPath+" n'existe pas ")
            valid = false
        }

        // si l'élément du dossier existe. 
        if valid == true {

            mode := fi.Mode();

            //si l'objet spécifié par le chemin est un dossier.
            if(mode.IsDir()==true){

                //Si l'utilisateur a oublié le "/" à la fin du chemin du fichier
                if(strings.LastIndexAny(newPath, "/") != len(newPath) - 1){
                    newPath=newPath+ string(os.PathSeparator)
                      
                }
                DecryptFolder(newPath, d)

            // si l'objet spécifié par le chemin est un fichier.
            }else if mode.IsRegular()== true {

                // si l'extension du fichier est différent de .gsh on peut chiffrer le fichier.
                if newPath[len(newPath)-4:]==".gsh"{
                	go DecryptFileAES(newPath, d, channelFolder, wgFolder)
                }     
            }
        }
    }

    // récupération des codes erreurs
    for _, entry := range entries {

        newPath := path+entry.Name()
        fi, err := os.Stat(newPath)
        valid := true
        if err != nil {
            fmt.Println("- \033[31mFailure Decryption\033[0m : "+newPath+" n'existe pas ")
            valid = false
        }

        // si l'élément du dossier existe. 
        if valid == true {

            mode := fi.Mode();

            if mode.IsRegular()== true {

                // si l'extension du fichier est différent de .gsh on peut chiffrer le fichier.
                if newPath[len(newPath)-4:]==".gsh"{
                    err := <- channelFolder 
					if err != nil {
						fmt.Println(err)
					}
                }     
            }
        }
    }

    // Attente que toutes les goroutines se terminent. 
    if countFiles!= 0 {
    	wgFolder.Wait()
    }

}

// DecryptFileFolder déchiffre les éléments choisis par l'utilisateur avec les données doc. 
func DecryptFileFolder(d *structure.Documents) {

	// Permettra de synchroniser le chiffrement des fichiers contenus dans le dossier. 
	wg := &sync.WaitGroup{}

	// Comptage des futures goruntines à lancer. 
	var countFiles int = 0
	for j:=0 ; j < len(d.Doc); j++ {
		stat, err := os.Stat(d.Doc[j])
		
		valid := true
        if err != nil {
            valid = false
        }

        if valid == true {
        	mode := stat.Mode()
        	if mode.IsRegular() == true {
        		if d.Doc[j][len(d.Doc[j])-4:]==".gsh"{
        			countFiles=countFiles+1
        		}
			}
        }
	}

	// Initialisation des goruntines
	wg.Add(countFiles)
	channel := make (chan error)

	// Pour chaque élément choisi par l'utilisateur, on le dechiffre. 
    for i := 0; i < len(d.Doc); i++ {

    	// Ouverture de l'élément. 
        fi, err := os.Stat(d.Doc[i])
        valid := true
        if err != nil {
            fmt.Println("- \033[31mFailure Decryption\033[0m : "+d.Doc[i]+" n'existe pas ")
            valid = false
        }

        // Si l'élément est valide. 
        if valid == true {
            mode := fi.Mode();

            // Si l'élément spécifié par le chemin est un dossier.
            if(mode.IsDir()==true){

                // Si l'utilisateur a oublié le "/" à la fin du chemin du fichier.
                if(strings.LastIndexAny(d.Doc[i], "/") != len(d.Doc[i]) - 1){
                  d.Doc[i]=d.Doc[i]+ string(os.PathSeparator)
                }

                // Déchiffrement du dossier.
                DecryptFolder(d.Doc[i], d)

            // si l'objet spécifié par le chemin est un fichier.
            }else if mode.IsRegular()== true {

            	if d.Doc[i][len(d.Doc[i])-4:]==".gsh"{
            		// Déchiffrement du fichier. 
                	go DecryptFileAES(d.Doc[i], d, channel, wg)
            	}
            }
        }
    }

    // Récupération des erreurs lors du chiffrement. 
    for j:=0 ; j < len(d.Doc); j++ {
		stat2, err := os.Stat(d.Doc[j])
		valid := true
        if err != nil {
            valid = false
        }
        if valid == true {
        	mode := stat2.Mode()
        	if mode.IsRegular() == true {
        		if d.Doc[j][len(d.Doc[j])-4:]==".gsh"{
        			err := <- channel 
					if err != nil {
						fmt.Println(err)
					}
        		}
			}	
        }
	}

	// Attente que toutes les goroutines se terminent. 
    if countFiles!= 0 {
    	wg.Wait()
    }

}

	
