<!-- TABLE OF CONTENTS -->
## Table of Contents

* [About the Project](#about-the-project)
* [Getting Started](#getting-started)
  * [Prerequisites](#prerequisites)
  * [Installation](#installation)
* [Contributing](#contributing)
* [License](#license)
* [Contact](#contact)

<!-- ABOUT THE PROJECT -->
## About The Project

This cloud computing base operator that consists of many different components designed with a microservices architecture,  can be deployed in an Openshift cluster in order to run different kinds of calculations that require a total consumption of the resources of specific worker nodes.  

Currently, it is being used to calculate a model that gives us a synthetic spectrum of uniformly rotating stars that is a far better representation of the observed data than a synthetic spectrum of non-rotating stars.  This result will be compared to the data acquired by the [ESA Hipparcos](http://www.esa.int/Our_Activities/Space_Science/Hipparcos_overview) and [ESA Gaia](http://sci.esa.int/gaia/) space missions, as well as those carried out by ground-based observatories.

A non-detailed design looks like:
![design](https://github.com/vega-project/ccb-operator/blob/master/img/openshift-based-HPC-design.png?raw=true)

<!-- COMPONENTS -->
## Components

The operator consists of different components, each responsible for trivial but specific tasks. 
* Dispatcher
* Worker
* Result-collector
* Janitor
* Dashboard (backend and frontend)

#### Dispatcher
The dispatcher is responsible for creating the calculations and assign them to workers. A calculation can be created either from adding a value in the database (Redis is currently used) or by creating a new one from the dashboard.

### Worker
This component is a deamonset that will choose a specific labeled node to run, with the purpose of executing the given commands. Currently each execution will run the atlas12 and synspec commands.

### Result-collector
This component is responsible for gathering the results of each completed calculation and organize them in an NFS storage.

### Janitor
Because of the big amount of calculations that can be created in the cluster, this component is responsible for deleting any of the calculations that passed the retention time.

### Dashboard
The dashboard consists of two components, backend, and frontend, which acts as a web UI interface in order for users to manage the calculations.

<!-- GETTING STARTED -->
## Getting Started

This is an example of how you may give instructions on setting up your project locally.
To get a local copy up and running follow these simple example steps.

### Prerequisites
* Openshift 4.x cluster
* Network file server (NFS) 

### Installation
1. Get a free API Key at [https://example.com](https://example.com)
2. Clone the repo and go to the folder
```sh
git clone https://github.com/vega-project/ccb-operator.git
cd ccb-operator
```
3. Start the deployment using your nfs IP
```sh
make deploy NFS_SERVER_IP=0.0.0.0
```

<!-- CONTRIBUTING -->
## Contributing

Contributions are what make the open source community such an amazing place to be learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<!-- LICENSE -->
## License

Distributed under the GNU General Public License v3.0. See `LICENSE` for more information.

<!-- CONTACT -->
## Contact

Nikolaos Moraitis - [@droslean](https://github.com/droslean/) - nmoraiti@redhat.com

Vega Project Link: [https://github.com/vega-project/](https://github.com/vega-project/)
