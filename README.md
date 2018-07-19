# terraform-provider-opennebula

[OpenNebula](https://opennebula.org/) provider for [Terraform](https://www.terraform.io/).
 
* Leverages [OpenNebula's XML/RPC API](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html) 
* Tested for versions 5.X


This is based on a project started by Runtastic, and has been enhanced by BlackBerry to allow for definition of these resource types:
* Virtual Machines
* Images
* VNET Reservations
* Security Groups

As well as data sources for:
* Images
* VNETs
* Security Groups

## DOCUMENTATION
See the project wiki page for usage and examples

## ROADMAP

The following list represent's all of OpenNebula's resources reachable through their API. The checked items are the ones that are fully functional and tested:

* [x] [onevm](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onevm)
* [x] [onetemplate](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onetemplate)
* [ ] [onehost](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onehost)
* [ ] [onecluster](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onecluster)
* [ ] [onegroup](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onegroup)
* [ ] [onevdc](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onevdc)
* [x] [onevnet](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onevnet)
* [ ] [oneuser](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#oneuser)
* [ ] [onedatastore](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onedatastore)
* [x] [oneimage](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#oneimage)
* [ ] [onemarket](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onemarket)
* [ ] [onemarketapp](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onemarketapp)
* [ ] [onevrouter](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onevrouter)
* [ ] [onezone](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onezone)
* [x] [onesecgroup](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#onesecgroup)
* [ ] [oneacl](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#oneacl)
* [ ] [oneacct](https://docs.opennebula.org/5.4/integration/system_interfaces/api.html#oneacct)


## Collaborators

- [Lorenzo Arribas](https://github.com/larribas)
- [Jason Tevnan](https://github.com/tnosaj)
- [Corey Melanson @ BlackBerry](https://github.com/cmelanson)

## Contributing
Bug reports and pull requests are welcome on GitHub at
https://github.com/cmelanson/terraform-provider-opennebula. This project is
intended to be a safe, welcoming space for collaboration, and contributors are
expected to adhere to the
[Contributor Covenant](http://contributor-covenant.org) code of conduct.

## License

The gem is available as open source under the terms of
the [MIT License](http://opensource.org/licenses/MIT).
