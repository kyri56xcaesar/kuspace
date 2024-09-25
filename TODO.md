Thesis title: "Shell environment on a kubernetes platform" <br>
Author: Chalvatzis Kyriakos 



# Day 2

## TODO LIST / ROADMAP
There is a lot to be done, so let's break it down.

# What is asked? 

I want to develop an K8S Operator "application/API" that will run on a Kubernetes (environment) and will provide resources to fast and easy deploy other services(?).

# todo: 
    define services and deployment method!
        -> manifests?
        -> containers?
        -> user choice?
        -> shell provided? (how many, how much)
        -> ???

# What specifications are there?
- It should be done in accordance to a shell environment and subsequently to a user environment upon which there is a user profile. 

- A user will have to login in order to enter and gain access. (and a register option)

(So there must be a "init" like "getty + login" program/process that will handle this. (This doesn't handle the "register" action, perhaps not even needed, start as root, add users (unix like)))

- How will this run as an Operator, how do operators work? 

- How can this be accessed? Pipelined if same node or Socket if different, via a CLI program? A service that will work with HTTP via a browser? Should do both, http service should invoke the cli api accordingly. 

- On later notice, would be cool if there could be graphics, drag and drop methods, etc... 

# What is needed?
- documentation on :
    -> linux users and shells, init->getty->login... etc(?), sh, bash, ...? <br>
    -> interprocess comms <br>
    -> k8s operators, how can one be built? (check OPERATOR SDK ) <br>
        - https://sdk.operatorframework.io/ <br>
        - https://www.fortytwo.io/post/make-your-life-easier-with-custom-kubernetes-operators <br>
    -> distribution (of these "system processes") and of "product processes" <br>
    -> monitoring of eveyrthing <br>


<hr>


# What will be used? <br>
As I've noticed operators are written in GO, therefore that should be the goto for this project. Perhaps some programs could be wrapped in C, Idk, we'll see(C). <br>

