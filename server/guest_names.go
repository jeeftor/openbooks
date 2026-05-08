package server

import (
	"fmt"
	"hash/fnv"
	"unicode"

	"github.com/google/uuid"
)

const (
	guestNameMaxLen       = 16
	guestNameBaseMaxLen   = 11
	guestNameSuffixLength = 4
)

var guestAdjectives = []string{
	"antsy",
	"baffled",
	"baggy",
	"bendy",
	"blobby",
	"booger",
	"bouncy",
	"briny",
	"bumpy",
	"burpy",
	"clammy",
	"clanky",
	"clumpy",
	"cranky",
	"crinkly",
	"crispy",
	"crusty",
	"damp",
	"dangly",
	"dizzy",
	"dopey",
	"drooly",
	"dusty",
	"eager",
	"flaky",
	"floppy",
	"foamy",
	"funky",
	"fuzzy",
	"gassy",
	"gloppy",
	"goofy",
	"goopy",
	"greasy",
	"grimy",
	"grody",
	"grubby",
	"gurgly",
	"gunky",
	"hairy",
	"honky",
	"itchy",
	"jiggly",
	"jumpy",
	"leaky",
	"lumpy",
	"mangy",
	"messy",
	"moist",
	"moldy",
	"mucky",
	"muggy",
	"mushy",
	"musty",
	"nervy",
	"noisy",
	"nubby",
	"oozy",
	"pasty",
	"pimply",
	"ploppy",
	"puffy",
	"pukey",
	"queasy",
	"rank",
	"raspy",
	"rubbery",
	"runny",
	"scabby",
	"scratchy",
	"shifty",
	"slimy",
	"sloppy",
	"sloshy",
	"slumpy",
	"smelly",
	"snarfy",
	"sneezy",
	"soggy",
	"splotchy",
	"squashy",
	"squirmy",
	"squeaky",
	"squishy",
	"sticky",
	"stinky",
	"stubby",
	"sweaty",
	"tacky",
	"tangy",
	"toasty",
	"tubby",
	"twitchy",
	"warty",
	"wiggly",
	"wobbly",
	"wonky",
	"yucky",
	"zesty",
	"zippy",
}

var guestAnimals = []string{
	"aardvark",
	"alpaca",
	"baboon",
	"badger",
	"barnacle",
	"bat",
	"beaver",
	"beetle",
	"blobfish",
	"boar",
	"buffalo",
	"camel",
	"capybara",
	"carp",
	"catfish",
	"chinchilla",
	"chicken",
	"clam",
	"cobra",
	"crab",
	"cricket",
	"dodo",
	"donkey",
	"duck",
	"eel",
	"emu",
	"ferret",
	"fish",
	"flamingo",
	"frog",
	"gecko",
	"gerbil",
	"gnat",
	"goat",
	"goose",
	"gopher",
	"hamster",
	"hedgehog",
	"hippo",
	"hyena",
	"iguana",
	"jellyfish",
	"koala",
	"lemming",
	"lemur",
	"llama",
	"lobster",
	"lizard",
	"maggot",
	"manatee",
	"marmot",
	"meerkat",
	"minnow",
	"mole",
	"mongoose",
	"moose",
	"moth",
	"mule",
	"narwhal",
	"newt",
	"opossum",
	"otter",
	"owl",
	"ox",
	"panda",
	"penguin",
	"pig",
	"pigeon",
	"platypus",
	"poodle",
	"porcupine",
	"possum",
	"pug",
	"quail",
	"raccoon",
	"rabbit",
	"rat",
	"rooster",
	"salamander",
	"seagull",
	"shrimp",
	"skunk",
	"sloth",
	"slug",
	"snail",
	"snake",
	"squid",
	"squirrel",
	"stoat",
	"tapir",
	"tarantula",
	"toad",
	"turkey",
	"turtle",
	"vulture",
	"walrus",
	"warthog",
	"weasel",
	"wombat",
	"worm",
	"yak",
}

func guestNameFromUUID(userID uuid.UUID) string {
	seed := hashGuestName(userID, 0)
	adjective := guestAdjectives[int(seed%uint64(len(guestAdjectives)))]
	animal := guestAnimals[int((seed/uint64(len(guestAdjectives)))%uint64(len(guestAnimals)))]

	return truncateGuestName(fmt.Sprintf("%s_%s", adjective, animal), guestNameMaxLen)
}

func guestNameCollisionSuffix(userID uuid.UUID) string {
	seed := hashGuestName(userID, 1)
	return fmt.Sprintf("%0*x", guestNameSuffixLength, seed%0x10000)
}

func usernameWithSuffix(baseUsername, suffix string) string {
	base := truncateGuestName(baseUsername, guestNameBaseMaxLen)
	return fmt.Sprintf("%s_%s", base, suffix)
}

func truncateGuestName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	return name[:maxLen]
}

func validGuestNamePart(part string) bool {
	if part == "" {
		return false
	}
	for _, char := range part {
		if char > unicode.MaxASCII || !(char >= 'a' && char <= 'z') {
			return false
		}
	}
	return true
}

func validGeneratedUsername(name string) bool {
	if name == "" || len(name) > guestNameMaxLen || name[0] == '_' {
		return false
	}
	for _, char := range name {
		if char > unicode.MaxASCII {
			return false
		}
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' {
			continue
		}
		return false
	}
	return true
}

func hashGuestName(userID uuid.UUID, salt byte) uint64 {
	hash := fnv.New64a()
	hash.Write([]byte{salt})
	hash.Write(userID[:])
	return hash.Sum64()
}
