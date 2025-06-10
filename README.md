# MovieBot

A discord bot made in Go with disgo api wrapper:
- go version -> 1.24.3
- disgo version -> 0.18.16

## What are you able to do with it
The bot itself was born with the idea to decide which movie to watch with your friends, so anyone can suggest at most
two movies, then when the dedicated command to extract the movies is launched, two films are picked up randomly.
As for these two, people have to vote them and see which one is going to be watched.

As for the slashcommands, you can:
- Setup the bot to work in a specific channel
- Add movies to a list referred to the user
- List the movies added by everyone
- Start a poll with the two randomly extracted movies extracted (admin only command) 

After a movie is declared as the winner, this will get blacklisted in order to know which movie has been already watched

## Note
I know it's still quite a primitive bot, it lacks of a delete/edit command, parametrization for other things,
but i'll update it whenever i have time.

