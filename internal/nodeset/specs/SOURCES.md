# Vendored companion specifications

Downloaded from OPCFoundation/UA-Nodeset. Update in lockstep with registry.go.

| Spec | Namespace URI | File | Upstream path | Commit |
|------|---------------|------|---------------|--------|
| DI | http://opcfoundation.org/UA/DI/ | Opc.Ua.Di.NodeSet2.xml | DI/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| IA | http://opcfoundation.org/UA/IA/ | Opc.Ua.IA.NodeSet2.xml | IA/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| Machinery | http://opcfoundation.org/UA/Machinery/ | Opc.Ua.Machinery.NodeSet2.xml | Machinery/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| ISA95-JobControl | http://opcfoundation.org/UA/ISA95-JOBCONTROL_V2/ | Opc.Ua.ISA95-JOBCONTROL.NodeSet2.xml | ISA95-JOBCONTROL/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| Machinery Jobs | http://opcfoundation.org/UA/Machinery/Jobs/ | Opc.Ua.Machinery.Jobs.NodeSet2.xml | Machinery/Jobs/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| MachineTool | http://opcfoundation.org/UA/MachineTool/ | Opc.Ua.MachineTool.NodeSet2.xml | MachineTool/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| IRDI | http://opcfoundation.org/UA/Dictionary/IRDI | Opc.Ua.IRDI.NodeSet2.xml | PADIM/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| PADIM | http://opcfoundation.org/UA/PADIM/ | Opc.Ua.PADIM.NodeSet2.xml | PADIM/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| PackML | http://opcfoundation.org/UA/PackML/ | Opc.Ua.PackML.NodeSet2.xml | PackML/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| Machinery ProcessValues | http://opcfoundation.org/UA/Machinery/ProcessValues/ | Opc.Ua.Machinery.ProcessValues.NodeSet2.xml | Machinery/ProcessValues/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| Robotics | http://opcfoundation.org/UA/Robotics/ | Opc.Ua.Robotics.NodeSet2.xml | Robotics/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| Scales | http://opcfoundation.org/UA/Scales/V2/ | Opc.Ua.Scales.NodeSet2.xml | Scales/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| ISA-95 | http://www.OPCFoundation.org/UA/2013/01/ISA95 | Opc.ISA95.NodeSet2.xml | ISA-95/ | c335f575ca77c025cdf5dc994b03411d093571ef |
| OPC UA Core (ns0) | http://opcfoundation.org/UA/ | Opc.Ua.NodeSet2.xml | Schema/ | c335f575ca77c025cdf5dc994b03411d093571ef |

**Note on ns0 (`Opc.Ua.NodeSet2.xml`):** base OPC UA NodeSet, version **1.05.07** (≥ 1.05.03, the
highest `RequiredModel` version any bundled companion spec declares). Fetched from `Schema/` at the
same commit as the companion specs. Unlike every other spec above, ns0 is **loaded for resolution but
intentionally NOT registered in registry.go (load-but-don't-list)**: it is injected into every catalog
so `OpcUa:*` references resolve/validate, yet it never appears in the catalog tree or search.
